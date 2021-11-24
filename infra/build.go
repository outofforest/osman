package infra

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/description"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/parser"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/libexec"
)

type cloneFromFn func(srcBuildKey types.BuildKey) (types.ImageManifest, error)

// NewBuilder creates new image builder
func NewBuilder(config config.Build, repo *Repository, storage storage.Driver, parser parser.Parser) *Builder {
	return &Builder{
		rebuild:     config.Rebuild,
		readyBuilds: map[types.BuildKey]bool{},
		repo:        repo,
		storage:     storage,
		parser:      parser,
	}
}

// Builder builds images
type Builder struct {
	rebuild     bool
	readyBuilds map[types.BuildKey]bool

	repo    *Repository
	storage storage.Driver
	parser  parser.Parser
}

// BuildFromFile builds image from spec file
func (b *Builder) BuildFromFile(ctx context.Context, specFile, name string, tags ...types.Tag) error {
	return b.buildFromFile(ctx, map[types.BuildKey]bool{}, specFile, name, tags...)
}

// Build builds images
func (b *Builder) Build(ctx context.Context, img *description.Descriptor) error {
	return b.build(ctx, map[types.BuildKey]bool{}, img)
}

func (b *Builder) buildFromFile(ctx context.Context, stack map[types.BuildKey]bool, specFile, name string, tags ...types.Tag) error {
	commands, err := b.parser.Parse(specFile)
	if err != nil {
		return err
	}
	return b.build(ctx, stack, description.Describe(name, tags, commands...))
}

func (b *Builder) build(ctx context.Context, stack map[types.BuildKey]bool, img *description.Descriptor) (retErr error) {
	if !types.IsNameValid(img.Name()) {
		return fmt.Errorf("name %s is invalid", img.Name())
	}
	tags := img.Tags()
	if len(tags) == 0 {
		tags = types.Tags{description.DefaultTag}
	}
	keys := make([]types.BuildKey, 0, len(tags))
	for _, tag := range tags {
		if !tag.IsValid() {
			return fmt.Errorf("tag %s is invalid", tag)
		}
		key := types.NewBuildKey(img.Name(), tag)
		if stack[key] {
			return fmt.Errorf("loop in dependencies detected on image %s", key)
		}
		stack[key] = true
		keys = append(keys, key)
	}

	buildID := types.NewBuildID()

	path, err := ioutil.TempDir("/tmp", "imagebuilder-*")
	if err != nil {
		return err
	}

	specDir := filepath.Join(path, ".specdir")

	var imgUnmount storage.UnmountFn
	defer func() {
		if err := umount(path); err != nil {
			if retErr != nil {
				retErr = err
			}
			return
		}
		if err := os.Remove(specDir); err != nil && !os.IsNotExist(err) {
			if retErr != nil {
				retErr = err
			}
			return
		}
		if imgUnmount != nil {
			if err := imgUnmount(); err != nil {
				if retErr != nil {
					retErr = err
				}
				return
			}
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			if retErr == nil {
				retErr = err
			}
			return
		}
		if retErr != nil {
			if err := b.storage.Drop(buildID); err != nil {
				retErr = err
			}
			return
		}
	}()

	var base bool
	if img.Name() == "fedora" {
		tags := img.Tags()
		if len(img.Tags()) != 1 {
			return errors.New("for fedora image exactly one tag is required")
		}
		if err := b.storage.CreateEmpty(img.Name(), buildID); err != nil {
			return err
		}
		var err error
		imgUnmount, err = b.storage.Mount(buildID, path)
		if err != nil {
			return err
		}

		if err := libexec.Exec(ctx,
			exec.Command("dnf", "install", "-y", "--installroot="+path, "--releasever="+string(tags[0]),
				"--setopt=install_weak_deps=False", "--setopt=keepcache=False", "--nodocs",
				"dnf", "langpacks-en")); err != nil {
			return err
		}
	}

	build := newImageBuild(base, path, func(srcBuildKey types.BuildKey) (types.ImageManifest, error) {
		if !types.IsNameValid(srcBuildKey.Name) {
			return types.ImageManifest{}, fmt.Errorf("name %s is invalid", srcBuildKey.Name)
		}
		if !srcBuildKey.Tag.IsValid() {
			return types.ImageManifest{}, fmt.Errorf("tag %s is invalid", srcBuildKey.Tag)
		}

		errRebuild := errors.New("rebuild")
		// Try to clone existing image
		err := errRebuild
		var srcBuildID types.BuildID
		if !b.rebuild || b.readyBuilds[srcBuildKey] {
			srcBuildID, err = b.storage.BuildID(srcBuildKey)
		}

		switch {
		case err == nil:
		case errors.Is(err, errRebuild) || errors.Is(err, storage.ErrSourceImageDoesNotExist):
			// If image does not exist try to build it from spec file in the current directory but only if tag is a default one
			if srcBuildKey.Tag == description.DefaultTag {
				err = b.buildFromFile(ctx, stack, srcBuildKey.Name, srcBuildKey.Name, description.DefaultTag)
			}
		default:
			return types.ImageManifest{}, err
		}

		switch {
		case err == nil:
		case errors.Is(err, errRebuild) || os.IsNotExist(err) || errors.Is(err, storage.ErrSourceImageDoesNotExist):
			// If spec file does not exist, try building from repository
			if baseImage := b.repo.Retrieve(srcBuildKey); baseImage != nil {
				err = b.build(ctx, stack, baseImage)
			} else {
				err = fmt.Errorf("can't find image %s: %w", srcBuildKey, err)
			}
		default:
			return types.ImageManifest{}, err
		}

		if err != nil {
			return types.ImageManifest{}, err
		}

		if !srcBuildID.IsValid() {
			srcBuildID, err = b.storage.BuildID(srcBuildKey)
			if err != nil {
				return types.ImageManifest{}, err
			}
		}

		if err := b.storage.Clone(srcBuildID, img.Name(), buildID); err != nil {
			return types.ImageManifest{}, err
		}

		imgUnmount, err = b.storage.Mount(buildID, path)
		if err != nil {
			return types.ImageManifest{}, err
		}

		manifest, err := b.storage.Manifest(srcBuildID)
		if err != nil {
			return types.ImageManifest{}, err
		}

		// To mount specdir readonly, trick is required:
		// 1. mount dir normally
		// 2. remount it using read-only option
		if err := os.Mkdir(specDir, 0o700); err != nil {
			return types.ImageManifest{}, err
		}
		if err := syscall.Mount(".", specDir, "", syscall.MS_BIND, ""); err != nil {
			return types.ImageManifest{}, err
		}
		if err := syscall.Mount(".", specDir, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
			return types.ImageManifest{}, err
		}

		if err := syscall.Mount("none", filepath.Join(path, "dev"), "devtmpfs", 0, ""); err != nil {
			return types.ImageManifest{}, err
		}
		if err := syscall.Mount("none", filepath.Join(path, "proc"), "proc", 0, ""); err != nil {
			return types.ImageManifest{}, err
		}
		if err := syscall.Mount("none", filepath.Join(path, "sys"), "sysfs", 0, ""); err != nil {
			return types.ImageManifest{}, err
		}
		return manifest, nil
	})

	for _, cmd := range img.Commands() {
		if err := cmd.Execute(ctx, build); err != nil {
			return err
		}
	}

	build.manifest.BuildID = buildID
	if err := b.storage.StoreManifest(build.manifest); err != nil {
		return err
	}

	for _, key := range keys {
		if err := b.storage.Tag(buildID, key.Tag); err != nil {
			return err
		}
	}
	for _, key := range keys {
		b.readyBuilds[key] = true
	}
	return nil
}

func umount(imgPath string) error {
	mountsRaw, err := ioutil.ReadFile("/proc/mounts")
	if err != nil {
		return err
	}
	for _, mount := range strings.Split(string(mountsRaw), "\n") {
		props := strings.SplitN(mount, " ", 3)
		if len(props) < 2 {
			// last empty line
			break
		}
		mountpoint := props[1]
		prefix := imgPath + "/"
		if !strings.HasPrefix(mountpoint, prefix) && mount != prefix {
			continue
		}
		if err := syscall.Unmount(mountpoint, 0); err != nil {
			return err
		}
	}
	return nil
}

func newImageBuild(base bool, path string, cloneFn cloneFromFn) *imageBuild {
	return &imageBuild{
		cloneFn: cloneFn,
		base:    base,
		path:    path,
	}
}

type imageBuild struct {
	cloneFn cloneFromFn

	base     bool
	path     string
	manifest types.ImageManifest
}

// from is a handler for FROM
func (b *imageBuild) From(cmd *description.FromCommand) error {
	if b.base {
		return errors.New("command FROM is forbidden for base image")
	}
	manifest, err := b.cloneFn(cmd.BuildKey)
	if err != nil {
		return err
	}
	b.manifest.BasedOn = manifest.BuildID
	b.manifest.Params = manifest.Params
	return nil
}

// params sets kernel params for image
func (b *imageBuild) Params(cmd *description.ParamsCommand) error {
	b.manifest.Params = append(b.manifest.Params, cmd.Params...)
	return nil
}

// run is a handler for RUN
func (b *imageBuild) Run(ctx context.Context, cmd *description.RunCommand) (retErr error) {
	root, err := os.Open("/")
	if err != nil {
		return err
	}
	defer root.Close()

	curr, err := os.Open(".")
	if err != nil {
		return err
	}
	defer curr.Close()

	if err := syscall.Chroot(b.path); err != nil {
		return err
	}
	if err := os.Chdir("/"); err != nil {
		return err
	}

	defer func() {
		defer func() {
			if err := curr.Chdir(); err != nil && retErr == nil {
				retErr = err
			}
		}()

		if err := root.Chdir(); err != nil {
			if retErr == nil {
				retErr = err
			}
			return
		}
		if err := syscall.Chroot("."); err != nil {
			if retErr == nil {
				retErr = err
			}
			return
		}
	}()

	return libexec.Exec(ctx, exec.Command("sh", "-c", cmd.Command))
}

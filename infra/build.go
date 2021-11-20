package infra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/google/uuid"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/runtime"

	"github.com/ridge/must"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/libexec"
)

const manifestFile = "manifest.json"

type cloneFromFn func(srcImageName string, srcTag types.Tag) (ImageManifest, error)

type buildKey struct {
	name string
	tag  types.Tag
}

func (bk buildKey) String() string {
	return fmt.Sprintf("%s:%s", bk.name, bk.tag)
}

var regExp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-_]*$`)

// IsTagValid returns true if tag is valid
func IsTagValid(tag types.Tag) bool {
	return regExp.MatchString(string(tag))
}

// IsNameValid returns true if name is valid
func IsNameValid(name string) bool {
	return regExp.MatchString(name)
}

// NewBuilder creates new image builder
func NewBuilder(config runtime.Config, repo *Repository, storage storage.Driver) *Builder {
	return &Builder{
		rebuild:     config.Rebuild,
		readyBuilds: map[buildKey]bool{},
		repo:        repo,
		storage:     storage,
	}
}

// Builder builds images
type Builder struct {
	rebuild     bool
	readyBuilds map[buildKey]bool

	repo    *Repository
	storage storage.Driver
}

// BuildFromFile builds image from spec file
func (b *Builder) BuildFromFile(ctx context.Context, specFile, name string, tags ...types.Tag) (imgBuild *ImageBuild, retErr error) {
	return b.buildFromFile(ctx, map[buildKey]bool{}, specFile, name, tags...)
}

// Build builds images
func (b *Builder) Build(ctx context.Context, img *Descriptor) (retBuild *ImageBuild, retErr error) {
	return b.build(ctx, map[buildKey]bool{}, img)
}

func (b *Builder) buildFromFile(ctx context.Context, stack map[buildKey]bool, specFile, name string, tags ...types.Tag) (imgBuild *ImageBuild, retErr error) {
	commands, err := Parse(specFile)
	if err != nil {
		return nil, err
	}
	return b.build(ctx, stack, Describe(name, tags, commands...))
}

func (b *Builder) build(ctx context.Context, stack map[buildKey]bool, img *Descriptor) (retBuild *ImageBuild, retErr error) {
	if !IsNameValid(img.Name()) {
		return nil, fmt.Errorf("name %s is invalid", img.Name())
	}
	tags := img.Tags()
	if len(tags) == 0 {
		tags = []types.Tag{runtime.DefaultTag}
	}
	keys := make([]buildKey, 0, len(tags))
	for _, tag := range tags {
		if !IsTagValid(tag) {
			return nil, fmt.Errorf("tag %s is invalid", tag)
		}
		key := buildKey{name: img.Name(), tag: tag}
		if stack[key] {
			return nil, fmt.Errorf("loop in dependencies detected on image %s", key)
		}
		stack[key] = true
		keys = append(keys, key)
	}

	buildID := types.BuildID(uuid.Must(uuid.NewUUID()).String())

	path, err := ioutil.TempDir("/tmp", "imagebuilder-*")
	if err != nil {
		return nil, err
	}

	specDir := filepath.Join(path, ".specdir")

	var imgUnmount storage.UnmountFn
	defer func() {
		if retErr != nil {
			retBuild = nil
		}
	}()
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
			return nil, errors.New("for fedora image exactly one tag is required")
		}
		if err := b.storage.CreateEmpty(img.Name(), buildID); err != nil {
			return nil, err
		}
		var err error
		imgUnmount, err = b.storage.Mount(buildID, path)
		if err != nil {
			return nil, err
		}

		if err := libexec.Exec(ctx,
			exec.Command("dnf", "install", "-y", "--installroot="+path, "--releasever="+string(tags[0]),
				"--setopt=install_weak_deps=False", "--setopt=keepcache=False", "--nodocs",
				"dnf", "langpacks-en")); err != nil {
			return nil, err
		}
	}

	build := newImageBuild(base, path, func(srcImageName string, srcTag types.Tag) (ImageManifest, error) {
		errRebuild := errors.New("rebuild")
		// Try to clone existing image
		var cloned bool
		err := errRebuild
		if !b.rebuild || b.readyBuilds[buildKey{name: srcImageName, tag: srcTag}] {
			err = b.storage.Clone(srcImageName, srcTag, img.Name(), buildID)
		}

		switch {
		case err == nil:
			cloned = true
		case errors.Is(err, errRebuild) || errors.Is(err, storage.ErrSourceImageDoesNotExist):
			// If image does not exist try to build it from spec file in the current directory but only if tag is a default one
			if srcTag == runtime.DefaultTag {
				_, err = b.buildFromFile(ctx, stack, srcImageName+".spec", srcImageName, runtime.DefaultTag)
			}
		default:
			return ImageManifest{}, err
		}

		switch {
		case err == nil:
		case errors.Is(err, errRebuild) || os.IsNotExist(err) || errors.Is(err, storage.ErrSourceImageDoesNotExist):
			// If spec file does not exist, try building from repository
			if baseImage := b.repo.Retrieve(srcImageName, srcTag); baseImage != nil {
				_, err = b.build(ctx, stack, baseImage)
			} else {
				err = fmt.Errorf("can't find image %s:%s", srcImageName, srcTag)
			}
		default:
			return ImageManifest{}, err
		}

		if err != nil {
			return ImageManifest{}, err
		}

		if !cloned {
			if err := b.storage.Clone(srcImageName, srcTag, img.Name(), buildID); err != nil {
				return ImageManifest{}, err
			}
		}

		imgUnmount, err = b.storage.Mount(buildID, path)
		if err != nil {
			return ImageManifest{}, err
		}

		manifestRaw, err := ioutil.ReadFile(filepath.Join(path, manifestFile))
		if err != nil {
			return ImageManifest{}, err
		}
		if err := os.Remove(filepath.Join(path, manifestFile)); err != nil {
			return ImageManifest{}, err
		}

		var manifest ImageManifest
		if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
			return ImageManifest{}, err
		}

		// To mount specdir readonly, trick is required:
		// 1. mount dir normally
		// 2. remount it using read-only option
		if err := os.Mkdir(specDir, 0o700); err != nil {
			return ImageManifest{}, err
		}
		if err := syscall.Mount(".", specDir, "", syscall.MS_BIND, ""); err != nil {
			return ImageManifest{}, err
		}
		if err := syscall.Mount(".", specDir, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
			return ImageManifest{}, err
		}

		if err := syscall.Mount("/dev", filepath.Join(path, "dev"), "", syscall.MS_BIND, ""); err != nil {
			return ImageManifest{}, err
		}
		if err := syscall.Mount("/proc", filepath.Join(path, "proc"), "", syscall.MS_BIND, ""); err != nil {
			return ImageManifest{}, err
		}
		if err := syscall.Mount("/sys", filepath.Join(path, "sys"), "", syscall.MS_BIND, ""); err != nil {
			return ImageManifest{}, err
		}
		return manifest, nil
	})

	for _, cmd := range img.commands {
		if err := cmd.execute(ctx, build); err != nil {
			return nil, err
		}
	}

	if err := ioutil.WriteFile(filepath.Join(path, manifestFile), must.Bytes(json.Marshal(build.Manifest())), 0o444); err != nil {
		return nil, err
	}

	for _, key := range keys {
		if err := b.storage.Tag(buildID, key.tag); err != nil {
			return nil, err
		}
	}
	build.manifest.BuildID = buildID
	for _, key := range keys {
		b.readyBuilds[key] = true
	}
	return build, nil
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

// ImageManifest contains info about built image
type ImageManifest struct {
	BuildID types.BuildID
	Params  []string
}

func newImageBuild(base bool, path string, cloneFn cloneFromFn) *ImageBuild {
	return &ImageBuild{
		cloneFn: cloneFn,
		base:    base,
		path:    path,
	}
}

// ImageBuild represents build of an image
type ImageBuild struct {
	cloneFn cloneFromFn

	base     bool
	path     string
	manifest ImageManifest
}

// Manifest returns image manifest
func (b *ImageBuild) Manifest() ImageManifest {
	return b.manifest
}

// from is a handler for FROM
func (b *ImageBuild) from(cmd *fromCommand) error {
	if b.base {
		return errors.New("command FROM is forbidden for base image")
	}
	manifest, err := b.cloneFn(cmd.imageName, cmd.tag)
	if err != nil {
		return err
	}
	b.manifest.Params = manifest.Params
	return nil
}

// params sets kernel params for image
func (b *ImageBuild) params(cmd *paramsCommand) error {
	b.manifest.Params = append(b.manifest.Params, cmd.params...)
	return nil
}

// run is a handler for RUN
func (b *ImageBuild) run(ctx context.Context, cmd *runCommand) (retErr error) {
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

	return libexec.Exec(ctx, exec.Command("sh", "-c", cmd.command))
}

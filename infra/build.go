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
	"strings"
	"syscall"

	"github.com/google/uuid"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/runtime"

	"github.com/otiai10/copy"
	"github.com/ridge/must"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/libexec"
)

const manifestFile = "manifest.json"

type cloneFromFn func(srcImageName string, srcTag types.Tag) (ImageManifest, error)

// BuildFromFile builds image from spec file
func BuildFromFile(ctx context.Context, builder *Builder, specFile string, tags ...types.Tag) (imgBuild *ImageBuild, retErr error) {
	commands, err := Parse(specFile)
	if err != nil {
		return nil, err
	}
	return builder.Build(ctx, Describe(strings.TrimSuffix(filepath.Base(specFile), ".spec"), tags, commands...))
}

// NewBuilder creates new image builder
func NewBuilder(repo *Repository, storage storage.Driver) *Builder {
	return &Builder{
		repo:    repo,
		storage: storage,
	}
}

// Builder builds images
type Builder struct {
	repo    *Repository
	storage storage.Driver
}

// Build builds images
func (b *Builder) Build(ctx context.Context, img *Descriptor) (imgBuild *ImageBuild, retErr error) {
	path, err := ioutil.TempDir("/tmp", "imagebuilder-*")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.Remove(path); retErr == nil && !os.IsNotExist(err) {
			retErr = err
		}
	}()

	var imgUnmount storage.UnmountFn
	defer func() {
		if err := umount(path); err != nil && retErr == nil {
			retErr = err
		}
		if imgUnmount != nil {
			if err := imgUnmount(); err != nil && retErr == nil {
				retErr = err
			}
		}
	}()
	if err := umount(path); err != nil {
		return nil, err
	}

	buildID := types.BuildID(uuid.Must(uuid.NewUUID()).String())
	var base bool
	if img.Name() == "fedora" {
		tags := img.Tags()
		if len(img.Tags()) != 1 {
			return nil, errors.New("for fedora image exactly one tag is required")
		}
		if err := b.storage.Create(img.Name(), buildID); err != nil {
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
		// Try to clone existing image
		var cloned bool
		err = b.storage.Clone(srcImageName, srcTag, img.Name(), buildID)

		switch {
		case err == nil:
			cloned = true
		case errors.Is(err, storage.ErrSourceImageDoesNotExist):
			// If image does not exist try to build it from spec file in the current directory but only if tag is a default one
			if srcTag == runtime.DefaultTag {
				_, err = BuildFromFile(ctx, b, srcImageName+".spec", runtime.DefaultTag)
			}
		default:
			return ImageManifest{}, err
		}

		switch {
		case err == nil:
		case os.IsNotExist(err) || errors.Is(err, storage.ErrSourceImageDoesNotExist):
			// If spec file does not exist, try building from repository
			if baseImage := b.repo.Retrieve(srcImageName, srcTag); baseImage != nil {
				_, err = b.Build(ctx, baseImage)
			} else {
				err = fmt.Errorf("can't find image %s", srcImageName)
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

	if err := b.storage.Tag(buildID, img.tags); err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(filepath.Join(path, manifestFile), must.Bytes(json.Marshal(build.Manifest())), 0o444); err != nil {
		return nil, err
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
		if !strings.HasPrefix(mountpoint, imgPath+"/") {
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
	Params []string
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

// copy is a handler for COPY
func (b *ImageBuild) copy(cmd *copyCommand) error {
	return copy.Copy(cmd.from, filepath.Join(b.path, cmd.to), copy.Options{PreserveTimes: true, PreserveOwner: true})
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

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

	"github.com/otiai10/copy"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/libexec"
)

const fromScratch = "scratch"

type cloneFromFn func(srcImageName string) error

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
	path, err := b.storage.Path(img.Name())
	if err != nil {
		return nil, err
	}

	if err := umount(path); err != nil {
		return nil, err
	}
	defer func() {
		err := umount(path)
		if err == nil && retErr != nil {
			_ = b.storage.Drop(img.Name())
			return
		}
		if retErr == nil {
			retErr = err
		}
	}()

	build := newImageBuild(path, b.clone(ctx, img.Name()))

	for _, cmd := range img.commands {
		if err := cmd.execute(ctx, build); err != nil {
			return nil, err
		}
	}
	return build, nil
}

func (b *Builder) clone(ctx context.Context, dstImageName string) cloneFromFn {
	return func(srcImageName string) error {
		dstImgPath, err := b.storage.Path(dstImageName)
		if err != nil {
			return err
		}
		if srcImageName == fromScratch {
			if err := b.storage.Create(dstImageName); err != nil {
				return err
			}

			err := libexec.Exec(ctx,
				exec.Command("dnf", "install", "-y", "--installroot="+dstImgPath, "--releasever=34",
					"--setopt=install_weak_deps=False", "--setopt=keepcache=False", "--nodocs",
					"dnf"))
			if err != nil {
				return err
			}
		} else {
			srcImgPath, err := b.storage.Path(srcImageName)
			if err != nil {
				return err
			}
			if err := umount(srcImgPath); err != nil {
				return err
			}
			err = b.storage.Clone(srcImageName, dstImageName)
			if err != nil && errors.Is(err, storage.ErrSourceImageDoesNotExist) {
				if img := b.repo.Retrieve(srcImageName); img != nil {
					if _, err := b.Build(ctx, img); err != nil {
						return err
					}
					err = b.storage.Clone(srcImageName, dstImageName)
				} else {
					return fmt.Errorf("image %s does not exist in repository", srcImageName)
				}
			}
			if err != nil {
				return err
			}
		}

		if err := syscall.Mount("/dev", dstImgPath+"/dev", "", syscall.MS_BIND, ""); err != nil {
			return err
		}
		if err := syscall.Mount("/proc", dstImgPath+"/proc", "", syscall.MS_BIND, ""); err != nil {
			return err
		}
		return syscall.Mount("/sys", dstImgPath+"/sys", "", syscall.MS_BIND, "")
	}
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

func newImageBuild(path string, cloneFn cloneFromFn) *ImageBuild {
	return &ImageBuild{
		cloneFn: cloneFn,
		path:    path,
		labels:  map[string]string{},
	}
}

// ImageBuild represents build of an image
type ImageBuild struct {
	cloneFn cloneFromFn

	path   string
	labels map[string]string
}

// Path returns path to directory where image is created
func (b *ImageBuild) Path() string {
	return b.path
}

// Label returns a value of label
func (b *ImageBuild) Label(name string) string {
	return b.labels[name]
}

// from is a handler for FROM
func (b *ImageBuild) from(cmd *fromCommand) error {
	return b.cloneFn(cmd.imageName)
}

// label is a handler for Label
func (b *ImageBuild) label(cmd *labelCommand) error {
	b.labels[cmd.name] = cmd.value
	return nil
}

// copy is a handler for COPY
func (b *ImageBuild) copy(cmd *copyCommand) error {
	return copy.Copy(cmd.from, filepath.Join(b.path, cmd.to))
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

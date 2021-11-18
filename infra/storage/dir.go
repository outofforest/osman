package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/otiai10/copy"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/runtime"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

const (
	subDirImages = "images"
	subDirBuilds = "builds"
)

// NewDirDriver returns new storage driver based on directories
func NewDirDriver(config runtime.Config) Driver {
	return &dirDriver{
		rootPath: config.RootDir,
	}
}

type dirDriver struct {
	rootPath string
}

// Mount mounts image in filesystem
func (d *dirDriver) Mount(buildID types.BuildID, dstPath string) (UnmountFn, error) {
	buildLink, err := d.toAbsoluteBuildLink(buildID)
	if err != nil {
		return nil, err
	}
	buildPath, err := filepath.EvalSymlinks(buildLink)
	if err != nil {
		return nil, err
	}

	if err := syscall.Mount(buildPath, dstPath, "", syscall.MS_BIND, ""); err != nil {
		return nil, err
	}

	return func() error {
		return syscall.Unmount(dstPath, 0)
	}, nil
}

// Create creates blank image
func (d *dirDriver) Create(imageName string, buildID types.BuildID) error {
	buildPath, err := d.toAbsoluteBuildDir(imageName, buildID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(buildPath, 0o700); err != nil {
		return err
	}

	dir := filepath.Join("..", d.toRelativeBuildDir(imageName, buildID))
	link, err := d.toAbsoluteBuildLink(buildID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil && !os.IsExist(err) {
		return err
	}
	return os.Symlink(dir, link)
}

// Clone clones source image to destination or returns false if source image does not exist
func (d *dirDriver) Clone(srcImageName string, srcTag types.Tag, dstImageName string, dstBuildID types.BuildID) error {
	dstImgPath, err := d.toAbsoluteBuildDir(dstImageName, dstBuildID)
	if err != nil {
		return err
	}

	srcImgLink, err := d.toAbsoluteTagLink(srcImageName, srcTag)
	if err != nil {
		return err
	}
	srcImgPath, err := filepath.EvalSymlinks(srcImgLink)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("image %s:%s does not exist: %w", srcImageName, srcTag, ErrSourceImageDoesNotExist)
		}
		return err
	}
	if err := d.Create(dstImageName, dstBuildID); err != nil {
		return err
	}
	return copy.Copy(srcImgPath, dstImgPath, copy.Options{PreserveTimes: true, PreserveOwner: true})
}

// Tag tags buildID with tags
func (d *dirDriver) Tag(buildID types.BuildID, tags []types.Tag) error {
	buildLink, err := d.toAbsoluteBuildLink(buildID)
	if err != nil {
		return err
	}
	buildDir, err := filepath.EvalSymlinks(buildLink)
	if err != nil {
		return err
	}
	parentDir := filepath.Dir(buildDir)
	for _, tag := range tags {
		tagLink := filepath.Join(parentDir, string(tag))
	retry:
		for {
			err := os.Symlink(string(buildID), tagLink)
			switch {
			case err == nil:
				break retry
			case os.IsExist(err):
				if err := os.Remove(tagLink); err != nil && !os.IsNotExist(err) {
					return err
				}
			default:
				return err
			}
		}
	}
	return nil
}

// Drop drops image
func (d *dirDriver) Drop(buildID types.BuildID) error {
	buildLinkPath, err := d.toAbsoluteBuildLink(buildID)
	if err != nil {
		return err
	}
	imgPath, err := filepath.EvalSymlinks(buildLinkPath)
	if err != nil {
		return err
	}

	if err := os.Remove(buildLinkPath); err != nil {
		return err
	}

	if err := os.RemoveAll(imgPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// FIXME (wojciech): remove tags

	return nil
}

func (d *dirDriver) toRelativeBuildDir(imageName string, buildID types.BuildID) string {
	return filepath.Join(subDirImages, imageName, string(buildID))
}

func (d *dirDriver) toAbsoluteBuildDir(imageName string, buildID types.BuildID) (string, error) {
	rootPath, err := filepath.Abs(d.rootPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, d.toRelativeBuildDir(imageName, buildID)), nil
}

func (d *dirDriver) toAbsoluteBuildLink(buildID types.BuildID) (string, error) {
	rootPath, err := filepath.Abs(d.rootPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, subDirBuilds, string(buildID)), nil
}

func (d *dirDriver) toAbsoluteTagLink(imageName string, tag types.Tag) (string, error) {
	rootPath, err := filepath.Abs(d.rootPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, subDirImages, imageName, string(tag)), nil
}

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

const subDirBuilds = "builds"

// FIXME (wojciech): remove once tags are introduced
const buildID types.BuildID = "id"

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
func (d *dirDriver) Mount(imageName string, buildID types.BuildID, dstPath string) (UnmountFn, error) {
	srcPath, err := d.toBuildPath(imageName, buildID)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(srcPath, 0o700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	if err := syscall.Mount(srcPath, dstPath, "", syscall.MS_BIND, ""); err != nil {
		return nil, err
	}

	return func() error {
		return syscall.Unmount(dstPath, 0)
	}, nil
}

// Clone clones source image to destination or returns false if source image does not exist
func (d *dirDriver) Clone(srcImageName string, dstImageName string, dstBuildID types.BuildID) error {
	dstImgPath, err := d.toBuildPath(dstImageName, dstBuildID)
	if err != nil {
		return err
	}

	if err := d.Drop(dstImageName, dstBuildID); err != nil {
		return err
	}

	srcImgPath, err := d.toBuildPath(srcImageName, buildID)
	if err != nil {
		return err
	}
	info, err := os.Stat(srcImgPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err != nil || !info.IsDir() {
		return fmt.Errorf("source image %s does not exist: %w", srcImageName, ErrSourceImageDoesNotExist)
	}
	return copy.Copy(srcImgPath, dstImgPath, copy.Options{PreserveTimes: true, PreserveOwner: true})
}

// Drop drops image
func (d *dirDriver) Drop(imageName string, buildID types.BuildID) error {
	imgPath, err := d.toBuildPath(imageName, buildID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(imgPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (d *dirDriver) toBuildPath(imageName string, buildID types.BuildID) (string, error) {
	rootPath, err := filepath.Abs(d.rootPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, imageName, subDirBuilds, string(buildID)), nil
}

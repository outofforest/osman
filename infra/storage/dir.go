package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/otiai10/copy"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/runtime"
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
func (d *dirDriver) Mount(imageName, dstPath string) (UnmountFn, error) {
	srcPath, err := d.toPath(imageName)
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
func (d *dirDriver) Clone(srcImageName string, dstImageName string) error {
	dstImgPath, err := d.toPath(dstImageName)
	if err != nil {
		return err
	}
	if err := d.Drop(dstImageName); err != nil {
		return err
	}

	srcImgPath, err := d.toPath(srcImageName)
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
	return copy.Copy(srcImgPath, dstImgPath)
}

// Drop drops image
func (d *dirDriver) Drop(imageName string) error {
	imgPath, err := d.toPath(imageName)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(imgPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (d *dirDriver) toPath(imageName string) (string, error) {
	rootPath, err := filepath.Abs(d.rootPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, imageName), nil
}

package storage

import (
	"errors"
)

// ErrSourceImageDoesNotExist is returned if source image does not exist
var ErrSourceImageDoesNotExist = errors.New("source image does not exist")

// UnmountFn unmounts mounted image
type UnmountFn = func() error

// Driver represents storage driver
type Driver interface {
	// Mount mounts the image in filesystem
	Mount(imageName, dstPath string) (UnmountFn, error)

	// Clone clones source image to destination
	Clone(srcImageName string, dstImageName string) error

	// Drop drops image
	Drop(imageName string) error
}

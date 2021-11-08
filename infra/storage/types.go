package storage

import "errors"

// ErrSourceImageDoesNotExist is returned if source image does not exist
var ErrSourceImageDoesNotExist = errors.New("source image does not exist")

// Driver represents storage driver
type Driver interface {
	// Path returns path to image
	Path(imageName string) (string, error)

	// Create creates destination path
	Create(dstImageName string) error

	// Clone clones source image to destination
	Clone(srcImageName string, dstImageName string) error

	// Drop drops image
	Drop(imageName string) error
}

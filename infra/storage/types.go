package storage

import (
	"errors"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// ErrSourceImageDoesNotExist is returned if source image does not exist
var ErrSourceImageDoesNotExist = errors.New("source image does not exist")

// UnmountFn unmounts mounted image
type UnmountFn = func() error

// Driver represents storage driver
type Driver interface {
	// Mount mounts the image in filesystem
	Mount(imageName string, buildID types.BuildID, dstPath string) (UnmountFn, error)

	// Clone clones source image to destination
	Clone(srcImageName string, dstImageName string, dstBuildID types.BuildID) error

	// Drop drops image
	Drop(imageName string, buildID types.BuildID) error
}

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
	Mount(buildID types.BuildID, dstPath string) (UnmountFn, error)

	// CreateEmpty creates blank image for build
	CreateEmpty(imageName string, buildID types.BuildID) error

	// Clone clones source image to destination
	Clone(srcImageName string, srcTag types.Tag, dstImageName string, dstBuildID types.BuildID) error

	// Tag tags buildID with tags
	Tag(buildID types.BuildID, tags []types.Tag) error

	// Drop drops image
	Drop(buildID types.BuildID) error
}

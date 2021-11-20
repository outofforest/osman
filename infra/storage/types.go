package storage

import (
	"errors"
	"time"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// ErrSourceImageDoesNotExist is returned if source image does not exist
var ErrSourceImageDoesNotExist = errors.New("source image does not exist")

// UnmountFn unmounts mounted image
type UnmountFn = func() error

// Driver represents storage driver
type Driver interface {
	// Builds returns available builds
	Builds() ([]types.BuildID, error)

	// Info returns information about build
	Info(buildID types.BuildID) (BuildInfo, error)

	// Mount mounts the build in filesystem
	Mount(buildID types.BuildID, dstPath string) (UnmountFn, error)

	// CreateEmpty creates blank build
	CreateEmpty(imageName string, buildID types.BuildID) error

	// Clone clones build to destination build
	Clone(srcBuildKey types.BuildKey, dstImageName string, dstBuildID types.BuildID) error

	// Tag tags build with tag
	Tag(buildID types.BuildID, tag types.Tag) error

	// Drop drops build
	Drop(buildID types.BuildID) error
}

// BuildInfo stores all the information about build
type BuildInfo struct {
	BuildID   types.BuildID
	CreatedAt time.Time
	Name      string
	Tags      types.Tags
}

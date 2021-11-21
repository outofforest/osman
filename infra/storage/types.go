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
	// Builds returns available builds
	Builds() ([]types.BuildID, error)

	// Info returns information about build
	Info(buildID types.BuildID) (types.BuildInfo, error)

	// BuildID returns build ID for build given by name and tag
	BuildID(buildKey types.BuildKey) (types.BuildID, error)

	// Mount mounts the build in filesystem
	Mount(buildID types.BuildID, dstPath string) (UnmountFn, error)

	// CreateEmpty creates blank build
	CreateEmpty(imageName string, buildID types.BuildID) error

	// Clone clones build to destination build
	Clone(srcBuildID types.BuildID, dstImageName string, dstBuildID types.BuildID) error

	// Manifest returns manifest of build
	Manifest(buildID types.BuildID) (types.ImageManifest, error)

	// StoreManifest stores manifest of build
	StoreManifest(manifest types.ImageManifest) error

	// Tag tags build with tag
	Tag(buildID types.BuildID, tag types.Tag) error

	// Drop drops build
	Drop(buildID types.BuildID) error
}

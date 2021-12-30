package storage

import (
	"errors"

	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/ioc/v2"
)

// ErrImageHasChildren is returned if image being deleted has children
var ErrImageHasChildren = errors.New("image has children")

// FinalizeFn unmounts mounted image
type FinalizeFn = func() error

// Driver represents storage driver
type Driver interface {
	// Builds returns available builds
	Builds() ([]types.BuildID, error)

	// Info returns information about build
	Info(buildID types.BuildID) (types.BuildInfo, error)

	// BuildID returns build ID for build given by name and tag
	BuildID(buildKey types.BuildKey) (types.BuildID, error)

	// CreateEmpty creates blank build
	CreateEmpty(imageName string, buildID types.BuildID) (FinalizeFn, string, error)

	// Clone clones build to destination build
	Clone(srcBuildID types.BuildID, dstImageName string, dstBuildID types.BuildID) (FinalizeFn, string, error)

	// Manifest returns manifest of build
	Manifest(buildID types.BuildID) (types.ImageManifest, error)

	// StoreManifest stores manifest of build
	StoreManifest(manifest types.ImageManifest) error

	// Tag tags build with tag
	Tag(buildID types.BuildID, tag types.Tag) error

	// Drop drops build
	Drop(buildID types.BuildID) error
}

// Resolve resolves concrete storage driver based on config
func Resolve(c *ioc.Container, config config.Storage) Driver {
	var driver Driver
	c.ResolveNamed(config.Driver, &driver)
	return driver
}

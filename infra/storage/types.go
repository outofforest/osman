package storage

import (
	"context"

	"github.com/pkg/errors"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/types"
)

// ErrImageHasChildren is returned if image being deleted has children.
var ErrImageHasChildren = errors.New("image has children")

// FinalizeFn unmounts mounted image.
type FinalizeFn = func() error

// Driver represents storage driver.
type Driver interface {
	// Builds returns available builds.
	Builds(ctx context.Context) ([]types.BuildID, error)

	// Info returns information about build.
	Info(ctx context.Context, buildID types.BuildID) (types.BuildInfo, error)

	// BuildID returns build ID for build given by name and tag.
	BuildID(ctx context.Context, buildKey types.BuildKey) (types.BuildID, error)

	// CreateEmpty creates blank build.
	CreateEmpty(ctx context.Context, imageName string, buildID types.BuildID) (FinalizeFn, string, error)

	// Clone clones build to destination build.
	Clone(
		ctx context.Context,
		srcBuildID types.BuildID,
		dstImageName string,
		dstBuildID types.BuildID,
	) (FinalizeFn, string, error)

	// StoreManifest stores manifest of build.
	StoreManifest(ctx context.Context, manifest types.ImageManifest) error

	// Tag tags build with tag.
	Tag(ctx context.Context, buildID types.BuildID, tag types.Tag) error

	// Untag removes tag from the build.
	Untag(ctx context.Context, buildID types.BuildID, tag types.Tag) error

	// Drop drops build.
	Drop(ctx context.Context, buildID types.BuildID) error
}

// Resolve resolves concrete storage driver based on config.
func Resolve(c *ioc.Container, config config.Storage) Driver {
	var driver Driver
	c.ResolveNamed(config.Driver, &driver)
	return driver
}

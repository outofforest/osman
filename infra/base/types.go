package base

import (
	"context"

	"github.com/outofforest/osman/infra/types"
)

// Initializer initializes base image
type Initializer interface {
	// Init installs base image inside directory
	Init(ctx context.Context, cacheDir, dir string, buildKey types.BuildKey) error
}

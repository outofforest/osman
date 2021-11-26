package base

import (
	"context"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// Initializer initializes base image
type Initializer interface {
	// Init installs base image inside directory
	Init(ctx context.Context, buildKey types.BuildKey, dstDir string) error
}

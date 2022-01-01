package base

import (
	"github.com/outofforest/osman/infra/types"
)

// Initializer initializes base image
type Initializer interface {
	// Init installs base image inside directory
	Init(dir string, buildKey types.BuildKey) error
}

package config

import (
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// DefaultTag is used if user specified empty tag list
const DefaultTag types.Tag = "latest"

// Build stores configuration for build command
type Build struct {
	// SpecFiles is the list of specfiles to build
	SpecFiles []string

	// Names is the list of names for corresponding specfiles
	Names []string

	// Tags are used to tag the build
	Tags []types.Tag

	// Rebuild forces rebuild of all parent images even if they exist
	Rebuild bool
}

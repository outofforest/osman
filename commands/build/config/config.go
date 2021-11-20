package config

import (
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/root/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// Build stores configuration for build command
type Build struct {
	config.Root

	// SpecFiles is the list of specfiles to build
	SpecFiles []string

	// Names is the list of names for corresponding specfiles
	Names []string

	// Tags are used to tag the build
	Tags []types.Tag

	// Rebuild forces rebuild of all parent images even if they exist
	Rebuild bool
}

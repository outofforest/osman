package runtime

import (
	"path/filepath"
	"strings"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// DefaultTag is used if user specified empty tag list
const DefaultTag types.Tag = "latest"

// NewConfigRootFactory creates new config factory common to all commands
func NewConfigRootFactory() *ConfigRootFactory {
	return &ConfigRootFactory{}
}

// ConfigRootFactory collects config common to all commands
type ConfigRootFactory struct {
	// RootDir is the root directory for images
	RootDir string

	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

// NewConfigBuildFactory creates new config factory specific for build command
func NewConfigBuildFactory() *ConfigBuildFactory {
	return &ConfigBuildFactory{}
}

// ConfigBuildFactory collects config specific for build command
type ConfigBuildFactory struct {
	// Names is the list of names for corresponding specfiles
	Names []string

	// Tags are used to tag the build
	Tags []string

	// Rebuild forces rebuild of all parent images even if they exist
	Rebuild bool
}

// NewConfigRoot builds config common to all commands
func NewConfigRoot(cf *ConfigRootFactory) ConfigRoot {
	return ConfigRoot{
		RootDir:        cf.RootDir,
		VerboseLogging: cf.VerboseLogging,
	}
}

// ConfigRoot stores configuration common to all commands
type ConfigRoot struct {
	// RootDir is the root directory for images
	RootDir string

	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

// NewConfigBuild builds config for base command
func NewConfigBuild(cf *ConfigBuildFactory, configRoot ConfigRoot, args Args) ConfigBuild {
	config := ConfigBuild{
		ConfigRoot: configRoot,
		SpecFiles:  args,
		Names:      cf.Names,
		Tags:       make([]types.Tag, 0, len(cf.Tags)),
		Rebuild:    cf.Rebuild,
	}

	for i, specFile := range config.SpecFiles {
		if len(config.Names) < i+1 {
			config.Names = append(config.Names, strings.TrimSuffix(filepath.Base(specFile), ".spec"))
		}
	}
	for _, tag := range cf.Tags {
		config.Tags = append(config.Tags, types.Tag(tag))
	}
	return config
}

// ConfigBuild stores configuration for build command
type ConfigBuild struct {
	ConfigRoot

	// SpecFiles is the list of specfiles to build
	SpecFiles []string

	// Names is the list of names for corresponding specfiles
	Names []string

	// Tags are used to tag the build
	Tags []types.Tag

	// Rebuild forces rebuild of all parent images even if they exist
	Rebuild bool
}

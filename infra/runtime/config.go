package runtime

import (
	"path/filepath"
	"strings"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// DefaultTag is used if user specified empty tag list
const DefaultTag types.Tag = "latest"

// NewConfigFactory creates new config factory
func NewConfigFactory() *ConfigFactory {
	return &ConfigFactory{}
}

// ConfigFactory produces config from parameters
type ConfigFactory struct {
	// RootDir is the root directory for images
	RootDir string

	// SpecFiles is the list of specfiles to build
	SpecFiles []string

	// Names is the list of names for corresponding specfiles
	Names []string

	// Tags are used to tag the build
	Tags []string

	// Rebuild forces rebuild of all parent images even if they exist
	Rebuild bool

	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

// NewConfigFromFactory builds final config from factory
func NewConfigFromFactory(cf *ConfigFactory) Config {
	config := Config{
		RootDir:        cf.RootDir,
		SpecFiles:      cf.SpecFiles,
		Names:          cf.Names,
		Tags:           make([]types.Tag, 0, len(cf.Tags)),
		Rebuild:        cf.Rebuild,
		VerboseLogging: cf.VerboseLogging,
	}

	config.SpecFiles = cf.SpecFiles
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

// Config stores configuration
type Config struct {
	// RootDir is the root directory for images
	RootDir string

	// SpecFiles is the list of specfiles to build
	SpecFiles []string

	// Names is the list of names for corresponding specfiles
	Names []string

	// Tags are used to tag the build
	Tags []types.Tag

	// Rebuild forces rebuild of all parent images even if they exist
	Rebuild bool

	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

package runtime

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ridge/must"
	"github.com/spf13/pflag"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// DefaultTag is used if user specified empty tag list
const DefaultTag types.Tag = "latest"

// NewConfigFactory creates new config factory
func NewConfigFactory() *ConfigFactory {
	cf := &ConfigFactory{}
	pflag.StringVar(&cf.RootDir, "root-dir", filepath.Join(must.String(os.UserHomeDir()), ".images"), "Directory where built images are stored")
	pflag.StringSliceVar(&cf.Names, "name", []string{}, "Name of built image, if empty name is derived from corresponding specfile")
	pflag.StringSliceVar(&cf.Tags, "tag", []string{string(DefaultTag)}, "Tags assigned to created build")
	pflag.BoolVar(&cf.Rebuild, "rebuild", false, "If set, all parent images are rebuilt even if they exist")
	pflag.BoolVarP(&cf.VerboseLogging, "verbose", "v", false, "Turns on verbose logging")
	pflag.Parse()
	return cf
}

// ConfigFactory produces config from parameters
type ConfigFactory struct {
	// RootDir is the root directory for images
	RootDir string

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
		SpecFiles:      make([]string, 0, pflag.NArg()),
		Names:          cf.Names,
		Tags:           make([]types.Tag, 0, len(cf.Tags)),
		Rebuild:        cf.Rebuild,
		VerboseLogging: cf.VerboseLogging,
	}

	for i := 0; i < pflag.NArg(); i++ {
		specFile := pflag.Arg(i)
		config.SpecFiles = append(config.SpecFiles, specFile)
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

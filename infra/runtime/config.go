package runtime

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"

	"github.com/ridge/must"
	"github.com/spf13/pflag"
)

// DefaultTag is used if user specified empty tag list
const DefaultTag types.Tag = "latest"

// NewConfigFromCLI returns configuration based on CLI arguments
func NewConfigFromCLI() Config {
	var config Config

	var tags []string
	pflag.StringVar(&config.RootDir, "root-dir", filepath.Join(must.String(os.UserHomeDir()), ".images"), "Directory where built images are stored")
	pflag.StringSliceVar(&config.Names, "name", []string{}, "Name of built image, if empty name is derived from corresponding specfile")
	pflag.StringSliceVar(&tags, "tag", []string{string(DefaultTag)}, "Tags assigned to created build")
	pflag.BoolVar(&config.Rebuild, "rebuild", false, "If set, all parent images are rebuilt even if they exist")
	pflag.BoolVarP(&config.VerboseLogging, "verbose", "v", false, "Turns on verbose logging")
	pflag.Parse()

	if pflag.NArg() == 0 {
		panic("at least one specfile has to be provided")
	}

	config.SpecFiles = make([]string, 0, pflag.NArg())
	for i := 0; i < pflag.NArg(); i++ {
		specFile := pflag.Arg(i)
		config.SpecFiles = append(config.SpecFiles, specFile)
		if len(config.Names) < i+1 {
			config.Names = append(config.Names, strings.TrimSuffix(filepath.Base(specFile), ".spec"))
		}
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

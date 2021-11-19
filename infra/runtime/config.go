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
	pflag.StringVar(&config.Name, "name", "", "Name of built image, if empty name is derived from spec file name")
	pflag.StringSliceVar(&tags, "tag", []string{string(DefaultTag)}, "Tags assigned to created build")
	pflag.BoolVar(&config.Rebuild, "rebuild", false, "If set, all parent images are rebuilt even if they exist")
	pflag.BoolVarP(&config.VerboseLogging, "verbose", "v", false, "Turns on verbose logging")
	pflag.Parse()

	if pflag.NArg() != 1 {
		panic("exactly one non-flag argument required pointing to image spec file")
	}

	config.SpecFile = pflag.Arg(0)
	if config.Name == "" {
		config.Name = strings.TrimSuffix(filepath.Base(config.SpecFile), ".spec")
	}
	return config
}

// Config stores configuration
type Config struct {
	// RootDir is the root directory for images
	RootDir string

	// SpecFile path to build
	SpecFile string

	// Name is the name for built image
	Name string

	// Tags are used to tag the build
	Tags []types.Tag

	// Rebuild forces rebuild of all parent images even if they exist
	Rebuild bool

	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

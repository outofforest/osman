package runtime

import (
	"os"
	"path/filepath"

	"github.com/ridge/must"
	"github.com/spf13/pflag"
)

// NewConfigFromCLI returns configuration based on CLI arguments
func NewConfigFromCLI() Config {
	var config Config

	pflag.StringVar(&config.RootDir, "root-dir", filepath.Join(must.String(os.UserHomeDir()), ".images"), "Directory where built images are stored")
	pflag.BoolVarP(&config.VerboseLogging, "verbose", "v", false, "Turns on verbose logging")
	pflag.Parse()

	if pflag.NArg() != 1 {
		panic("exactly one non-flag argument required pointing to image spec file")
	}

	config.Specfile = pflag.Arg(0)

	return config
}

// Config stores configuration
type Config struct {
	// RootDir is the root directory for images
	RootDir string

	// Specfile path to build
	Specfile string

	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"

	"github.com/ridge/must"
	"github.com/spf13/pflag"
)

// DefaultTag is used if user specified empty tag list
const DefaultTag types.Tag = "latest"

var tagRegExp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-_]*$`)

// IsTagValid returns true if tag is valid
func IsTagValid(tag types.Tag) bool {
	return tagRegExp.MatchString(string(tag))
}

// NewConfigFromCLI returns configuration based on CLI arguments
func NewConfigFromCLI() Config {
	var config Config

	var tags []string
	pflag.StringVar(&config.RootDir, "root-dir", filepath.Join(must.String(os.UserHomeDir()), ".images"), "Directory where built images are stored")
	pflag.StringSliceVar(&tags, "tag", []string{string(DefaultTag)}, "Tags assigned to created build")
	pflag.BoolVarP(&config.VerboseLogging, "verbose", "v", false, "Turns on verbose logging")
	pflag.Parse()

	if pflag.NArg() != 1 {
		panic("exactly one non-flag argument required pointing to image spec file")
	}

	config.Specfile = pflag.Arg(0)

	for _, tag := range tags {
		t := types.Tag(tag)
		if !IsTagValid(t) {
			panic(fmt.Errorf("tag '%s' is invalid", tag))
		}
		config.Tags = append(config.Tags, t)
	}
	if len(config.Tags) == 0 {
		config.Tags = []types.Tag{DefaultTag}
	}
	return config
}

// Config stores configuration
type Config struct {
	// RootDir is the root directory for images
	RootDir string

	// Specfile path to build
	Specfile string

	// Tags are used to tag the build
	Tags []types.Tag

	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

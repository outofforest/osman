package build

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/build/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/ioc"
)

// Install installs build command
func Install(c *ioc.Container) {
	c.Singleton(func() *configFactory {
		return &configFactory{}
	})
	c.Singleton(newConfig)
	c.TransientNamed("build", command)
}

type configFactory struct {
	// Names is the list of names for corresponding specfiles
	Names []string

	// Tags are used to tag the build
	Tags []string

	// Rebuild forces rebuild of all parent images even if they exist
	Rebuild bool
}

func newConfig(cf *configFactory, args commands.Args) config.Build {
	config := config.Build{
		SpecFiles: args,
		Names:     cf.Names,
		Tags:      make([]types.Tag, 0, len(cf.Tags)),
		Rebuild:   cf.Rebuild,
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

func command(cf *configFactory, cmdF *commands.CmdFactory) *cobra.Command {
	cmd := &cobra.Command{
		Short: "Builds images from spec files",
		Args:  cobra.MinimumNArgs(1),
		Use:   "build [flags] ...specfile",
		RunE:  cmdF.Cmd(imagebuilder.Build),
	}
	cmd.Flags().StringSliceVar(&cf.Names, "name", []string{}, "Name of built image, if empty name is derived from corresponding specfile")
	cmd.Flags().StringSliceVar(&cf.Tags, "tag", []string{string(config.DefaultTag)}, "Tags assigned to created build")
	cmd.Flags().BoolVar(&cf.Rebuild, "rebuild", false, "If set, all parent images are rebuilt even if they exist")
	return cmd
}

package root

import (
	"os"
	"path/filepath"

	"github.com/ridge/must"
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/root/config"
	"github.com/wojciech-malota-wojcik/ioc"
)

// Install installs root command
func Install(c *ioc.Container) {
	c.Singleton(func() *configFactory {
		return &configFactory{}
	})
	c.Singleton(func(cf *configFactory) config.Root {
		return config.Root{
			RootDir:        cf.RootDir,
			VerboseLogging: cf.VerboseLogging,
		}
	})
	c.Transient(command)
}

type configFactory struct {
	// RootDir is the root directory for images
	RootDir string

	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

func command(c *ioc.Container, cf *configFactory) *cobra.Command {
	rootCmd := &cobra.Command{
		SilenceUsage: true,
	}
	rootCmd.PersistentFlags().StringVar(&cf.RootDir, "root-dir", filepath.Join(must.String(os.UserHomeDir()), ".images"), "Directory where built images are stored")
	rootCmd.PersistentFlags().BoolVarP(&cf.VerboseLogging, "verbose", "v", false, "Turns on verbose logging")

	c.ForEachNamed(func(cmd *cobra.Command) {
		rootCmd.AddCommand(cmd)
	})

	return rootCmd
}

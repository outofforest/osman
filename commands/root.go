package commands

import (
	"os"
	"path/filepath"

	"github.com/ridge/must"
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/ioc/v2"
)

// NewRootCommand returns new root command
func NewRootCommand(c *ioc.Container, rootF *config.RootFactory, storageF *config.StorageFactory) *cobra.Command {
	rootCmd := &cobra.Command{
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	rootCmd.PersistentFlags().StringVar(&storageF.RootDir, "root-dir", filepath.Join(must.String(os.UserHomeDir()), ".images"), "Directory where built images are stored")
	rootCmd.PersistentFlags().BoolVarP(&rootF.VerboseLogging, "verbose", "v", false, "Turns on verbose logging")

	c.ForEachNamed(func(cmd *cobra.Command) {
		rootCmd.AddCommand(cmd)
	})

	return rootCmd
}

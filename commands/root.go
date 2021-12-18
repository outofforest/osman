package commands

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ridge/must"
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/ioc/v2"
)

// NewRootCommand returns new root command
func NewRootCommand(c *ioc.Container, rootF *config.RootFactory, storageF *config.StorageFactory) *cobra.Command {
	rootCmd := &cobra.Command{
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	rootCmd.PersistentFlags().StringVar(&storageF.Root, "storage-root", filepath.Join(must.String(os.UserHomeDir()), ".images"), "Location where built images are stored")
	rootCmd.PersistentFlags().StringVar(&storageF.Driver, "storage-driver", "dir", "Storage driver to use: "+strings.Join(c.Names((*storage.Driver)(nil)), " | "))
	rootCmd.PersistentFlags().BoolVarP(&rootF.VerboseLogging, "verbose", "v", false, "Turns on verbose logging")

	c.ForEachNamed(func(cmd *cobra.Command) {
		rootCmd.AddCommand(cmd)
	})

	return rootCmd
}

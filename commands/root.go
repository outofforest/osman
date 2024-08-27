package commands

import (
	"github.com/spf13/cobra"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/logger"
)

// NewRootCommand returns new root command.
func NewRootCommand(c *ioc.Container) *cobra.Command {
	rootCmd := &cobra.Command{
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	logger.AddFlags(logger.DefaultConfig, rootCmd.PersistentFlags())
	c.ForEachNamed(func(cmd *cobra.Command) {
		rootCmd.AddCommand(cmd)
	})
	return rootCmd
}

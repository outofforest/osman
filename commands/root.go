package commands

import (
	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/logger"
	"github.com/spf13/cobra"
)

// NewRootCommand returns new root command
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

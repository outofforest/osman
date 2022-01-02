package commands

import (
	"fmt"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/osman"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/description"
	"github.com/outofforest/osman/infra/format"
	"github.com/outofforest/osman/infra/types"
	"github.com/spf13/cobra"
)

// NewBuildCommand creates new build command
func NewBuildCommand(cmdF *CmdFactory) *cobra.Command {
	var loggingF *config.LoggingFactory
	var storageF *config.StorageFactory
	var formatF *config.FormatFactory
	buildF := &config.BuildFactory{}

	cmd := &cobra.Command{
		Short: "Builds images from spec files",
		Args:  cobra.MinimumNArgs(1),
		Use:   "build [flags] ...specfile",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
			c.Singleton(loggingF.Config)
			c.Singleton(storageF.Config)
			c.Singleton(formatF.Config)
			c.Singleton(buildF.Config)
		}, func(c *ioc.Container, formatter format.Formatter) error {
			var builds []types.BuildInfo
			var err error
			c.Call(osman.Build, &builds, &err)
			if err != nil {
				return err
			}
			fmt.Println(formatter.Format(builds))
			return nil
		}),
	}
	loggingF = cmdF.AddLoggingFlags(cmd)
	storageF = cmdF.AddStorageFlags(cmd)
	formatF = cmdF.AddFormatFlags(cmd)
	cmd.Flags().StringSliceVar(&buildF.Names, "name", []string{}, "Name of built image, if empty name is derived from corresponding specfile")
	cmd.Flags().StringSliceVar(&buildF.Tags, "tag", []string{string(description.DefaultTag)}, "Tags assigned to created build")
	cmd.Flags().BoolVar(&buildF.Rebuild, "rebuild", false, "If set, all parent images are rebuilt even if they exist")
	return cmd
}

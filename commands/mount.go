package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/osman"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/format"
	"github.com/outofforest/osman/infra/types"
)

// NewMountCommand creates new mount command.
func NewMountCommand(cmdF *CmdFactory) *cobra.Command {
	var storageF *config.StorageFactory
	var filterF *config.FilterFactory
	var formatF *config.FormatFactory
	mountF := &config.MountFactory{}

	cmd := &cobra.Command{
		Short: "Mounts image",
		Args:  cobra.RangeArgs(1, 2),
		Use:   "mount [flags] image [name][:tag]",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
			c.Singleton(storageF.Config)
			c.Singleton(filterF.Config)
			c.Singleton(formatF.Config)
			c.Singleton(mountF.Config)
		}, func(c *ioc.Container, formatter format.Formatter) error {
			var builds []types.BuildInfo
			var err error
			c.Call(osman.Mount, &builds, &err)
			if err != nil {
				return err
			}
			fmt.Println(formatter.Format(builds, defaultFields...))
			return nil
		}),
	}
	storageF = cmdF.AddStorageFlags(cmd)
	filterF = cmdF.AddFilterFlags(cmd, []string{config.BuildTypeImage})
	formatF = cmdF.AddFormatFlags(cmd)
	cmd.Flags().StringSliceVar(&mountF.Tags, "tag", []string{}, "Tags to be applied on mounts")
	cmd.Flags().BoolVar(&mountF.Boot, "boot", false, "Create mount used to boot host machine")
	return cmd
}

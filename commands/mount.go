package commands

import (
	"fmt"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/osman"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/format"
	"github.com/outofforest/osman/infra/types"
	"github.com/spf13/cobra"
)

// NewMountCommand creates new mount command
func NewMountCommand(cmdF *CmdFactory) *cobra.Command {
	var loggingF *config.LoggingFactory
	var storageF *config.StorageFactory
	var formatF *config.FormatFactory

	cmd := &cobra.Command{
		Short: "Mounts image",
		Args:  cobra.MinimumNArgs(2),
		Use:   "mount [flags] image name",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
			c.Singleton(loggingF.Config)
			c.Singleton(storageF.Config)
			c.Singleton(formatF.Config)
			c.Singleton(config.NewMount)
		}, func(c *ioc.Container, formatter format.Formatter) error {
			var build types.BuildInfo
			var err error
			c.Call(osman.Mount, &build, &err)
			if err != nil {
				return err
			}
			fmt.Println(formatter.Format(build))
			return nil
		}),
	}
	loggingF = cmdF.AddLoggingFlags(cmd)
	storageF = cmdF.AddStorageFlags(cmd)
	formatF = cmdF.AddFormatFlags(cmd)
	return cmd
}

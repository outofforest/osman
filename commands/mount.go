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
	mountF := &config.MountFactory{}

	cmd := &cobra.Command{
		Short: "Mounts VM in Libvirt",
		Args:  cobra.RangeArgs(1, 2),
		Use:   "mount [flags] image [vm-file]",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
			c.Singleton(loggingF.Config)
			c.Singleton(storageF.Config)
			c.Singleton(formatF.Config)
			c.Singleton(mountF.Config)
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
	cmd.Flags().StringVar(&mountF.LibvirtAddr, "libvirt-addr", "unix:///var/run/libvirt/libvirt-sock", "Address libvirt listens on")
	cmd.Flags().StringVar(&mountF.XMLDir, "xml-dir", "/tank/master/vms", "Directory where VM definition is taken from if vm-file argument is not provided")
	return cmd
}

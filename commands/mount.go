package commands

import (
	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/osman"
	"github.com/outofforest/osman/config"
	"github.com/spf13/cobra"
)

// NewMountCommand creates new mount command
func NewMountCommand(cmdF *CmdFactory) *cobra.Command {
	var loggingF *config.LoggingFactory
	var storageF *config.StorageFactory
	mountF := &config.MountFactory{}

	cmd := &cobra.Command{
		Short: "Mounts VM in Libvirt",
		Args:  cobra.RangeArgs(1, 2),
		Use:   "mount [flags] image [vm-file]",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
			c.Singleton(loggingF.Config)
			c.Singleton(storageF.Config)
			c.Singleton(mountF.Config)
		}, osman.Mount),
	}
	loggingF = cmdF.AddLoggingFlags(cmd)
	storageF = cmdF.AddStorageFlags(cmd)
	cmd.Flags().StringVar(&mountF.LibvirtAddr, "libvirt-addr", "unix:///var/run/libvirt/libvirt-sock", "Address libvirt listens on")
	cmd.Flags().StringVar(&mountF.XMLDir, "xml-dir", "/tank/master/vms", "Directory where VM definition is taken from if vm-file argument is not provided")
	return cmd
}

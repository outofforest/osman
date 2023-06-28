package commands

import (
	"fmt"
	"os"

	"github.com/outofforest/ioc/v2"
	"github.com/ridge/must"
	"github.com/spf13/cobra"

	"github.com/outofforest/osman"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/format"
	"github.com/outofforest/osman/infra/types"
)

// NewStartCommand creates new start command
func NewStartCommand(cmdF *CmdFactory) *cobra.Command {
	var storageF *config.StorageFactory
	var filterF *config.FilterFactory
	var formatF *config.FormatFactory
	startF := &config.StartFactory{}

	cmd := &cobra.Command{
		Short: "Starts VM",
		Args:  cobra.RangeArgs(1, 2),
		Use:   "start [flags] image [name][:tag]",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
			c.Singleton(storageF.Config)
			c.Singleton(filterF.Config)
			c.Singleton(formatF.Config)
			c.Singleton(startF.Config)
		}, func(c *ioc.Container, formatter format.Formatter) error {
			var builds []types.BuildInfo
			var err error
			c.Call(osman.Start, &builds, &err)
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
	cmd.Flags().StringVar(&startF.Tag, "tag", "", "Tag to be applied on VMs")
	cmd.Flags().StringVar(&startF.LibvirtAddr, "libvirt-addr", "unix:///var/run/libvirt/libvirt-sock", "Address libvirt listens on")
	cmd.Flags().StringVar(&startF.XMLDir, "xml-dir", must.String(os.UserHomeDir())+"/osman", "Directory where VM definition is taken from if vm-file argument is not provided")
	cmd.Flags().StringVar(&startF.VolumeDir, "volume-dir", "/tank/vms", "Directory where vm-specific folder exists containing subfolders to be mounted as filesystems in the VM")
	return cmd
}

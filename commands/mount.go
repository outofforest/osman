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

// NewMountCommand creates new mount command
func NewMountCommand(cmdF *CmdFactory) *cobra.Command {
	var storageF *config.StorageFactory
	var formatF *config.FormatFactory
	mountF := &config.MountFactory{}

	cmd := &cobra.Command{
		Short: "Mounts image",
		Args:  cobra.RangeArgs(1, 2),
		Use:   "mount [flags] image [name][:tag]",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
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
	storageF = cmdF.AddStorageFlags(cmd)
	formatF = cmdF.AddFormatFlags(cmd)
	cmd.Flags().StringVar(&mountF.LibvirtAddr, "libvirt-addr", "unix:///var/run/libvirt/libvirt-sock", "Address libvirt listens on")
	cmd.Flags().StringVar(&mountF.XMLDir, "xml-dir", must.String(os.UserHomeDir())+"/osman", "Directory where VM definition is taken from if vm-file argument is not provided")
	cmd.Flags().StringVar(&mountF.VMFile, "vm", "", "Defines VM for mounted image in Libvirt using provided file. If flag is provided without value or value is `auto` then file is derived as <xml-dir>/<image-name>.xml")
	cmd.Flags().Lookup("vm").NoOptDefVal = "auto"
	return cmd
}

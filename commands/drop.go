package commands

import (
	"errors"
	"fmt"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/osman"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/format"
	"github.com/spf13/cobra"
)

// NewDropCommand returns new drop command
func NewDropCommand(cmdF *CmdFactory) *cobra.Command {
	var loggingF *config.LoggingFactory
	var storageF *config.StorageFactory
	var filterF *config.FilterFactory
	var formatF *config.FormatFactory
	dropF := &config.DropFactory{}

	cmd := &cobra.Command{
		Short: "Drops builds",
		Use:   "drop [flags] [... buildID | [name][:tag]]",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
			c.Singleton(loggingF.Config)
			c.Singleton(storageF.Config)
			c.Singleton(filterF.Config)
			c.Singleton(formatF.Config)
			c.Singleton(dropF.Config)
		}, func(c *ioc.Container, formatter format.Formatter) error {
			var results []osman.Result
			var err error
			c.Call(osman.Drop, &results, &err)
			if err != nil {
				return err
			}
			err = nil
			for _, r := range results {
				if r.Result != nil {
					err = errors.New("some drops failed")
					break
				}
			}
			fmt.Println(formatter.Format(results))
			return err
		}),
	}
	loggingF = cmdF.AddLoggingFlags(cmd)
	storageF = cmdF.AddStorageFlags(cmd)
	filterF = cmdF.AddFilterFlags(cmd, []string{config.BuildTypeImage})
	formatF = cmdF.AddFormatFlags(cmd)
	cmd.Flags().StringVar(&dropF.LibvirtAddr, "libvirt-addr", "unix:///var/run/libvirt/libvirt-sock", "Address libvirt listens on")
	cmd.Flags().BoolVar(&dropF.All, "all", false, "It is required to set this flag to drop builds if no filters are provided")
	return cmd
}

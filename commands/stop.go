//nolint:dupl
package commands

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/osman"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/format"
)

// NewStopCommand creates new stop command.
func NewStopCommand(cmdF *CmdFactory) *cobra.Command {
	var storageF *config.StorageFactory
	var filterF *config.FilterFactory
	var formatF *config.FormatFactory
	stopF := &config.StopFactory{}

	cmd := &cobra.Command{
		Short: "Stops VMs",
		Use:   "stop [flags] [name][:tag]",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
			c.Singleton(storageF.Config)
			c.Singleton(filterF.Config)
			c.Singleton(formatF.Config)
			c.Singleton(stopF.Config)
		}, func(c *ioc.Container, formatter format.Formatter) error {
			var results []osman.Result
			var err error
			c.Call(osman.Stop, &results, &err)
			if err != nil {
				return err
			}
			err = nil
			for _, r := range results {
				if r.Result != nil {
					err = errors.New("some stops failed")
					break
				}
			}
			fmt.Println(formatter.Format(results))
			return nil
		}),
	}
	storageF = cmdF.AddStorageFlags(cmd)
	filterF = cmdF.AddFilterFlags(cmd, []string{config.BuildTypeVM})
	formatF = cmdF.AddFormatFlags(cmd)
	cmd.Flags().StringVar(&stopF.LibvirtAddr, "libvirt-addr", "unix:///var/run/libvirt/libvirt-sock",
		"Address libvirt listens on")
	cmd.Flags().BoolVar(&stopF.All, "all", false,
		"It is required to set this flag to stop builds if no filters are provided")
	return cmd
}

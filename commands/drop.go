package commands

import (
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder"
	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/format"
	"github.com/wojciech-malota-wojcik/ioc"
)

// NewDropCommand returns new drop command
func NewDropCommand(filterF *config.FilterFactory, dropF *config.DropFactory, cmdF *CmdFactory) *cobra.Command {
	cmd := &cobra.Command{
		Short: "Drops builds",
		Use:   "drop [flags] [... buildID | [name][:tag]]",
		RunE: cmdF.Cmd(func(c *ioc.Container, formatter format.Formatter) error {
			var err error
			c.Call(imagebuilder.Drop, &err)
			return err
		}),
	}
	cmd.Flags().BoolVar(&filterF.Untagged, "untagged", false, "If set, only untagged builds are deleted")
	cmd.Flags().BoolVar(&dropF.All, "all", false, "It is required to set this flag to drop builds if no filters are provided")
	return cmd
}

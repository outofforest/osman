package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder"
	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/format"
	"github.com/wojciech-malota-wojcik/ioc/v2"
)

// NewDropCommand returns new drop command
func NewDropCommand(c *ioc.Container, filterF *config.FilterFactory, dropF *config.DropFactory, formatF *config.FormatFactory, cmdF *CmdFactory) *cobra.Command {
	cmd := &cobra.Command{
		Short: "Drops builds",
		Use:   "drop [flags] [... buildID | [name][:tag]]",
		RunE: cmdF.Cmd(func(c *ioc.Container, formatter format.Formatter) error {
			var results []imagebuilder.Result
			var err error
			c.Call(imagebuilder.Drop, &results, &err)
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
	cmd.Flags().BoolVar(&filterF.Untagged, "untagged", false, "If set, only untagged builds are deleted")
	cmd.Flags().BoolVar(&dropF.All, "all", false, "It is required to set this flag to drop builds if no filters are provided")
	cmd.Flags().StringVar(&formatF.Formatter, "format", "table", "Name of formatter used to format the output: "+strings.Join(c.Names((*format.Formatter)(nil)), " | "))
	return cmd
}

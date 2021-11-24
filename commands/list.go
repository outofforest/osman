package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder"
	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/format"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/ioc/v2"
)

// NewListCommand returns new list command
func NewListCommand(c *ioc.Container, filterF *config.FilterFactory, formatF *config.FormatFactory, cmdF *CmdFactory) *cobra.Command {
	cmd := &cobra.Command{
		Short: "Lists information about available builds",
		Use:   "list [flags] [... buildID | [name][:tag]]",
		RunE: cmdF.Cmd(func(c *ioc.Container, formatter format.Formatter) error {
			var builds []types.BuildInfo
			var err error
			c.Call(imagebuilder.List, &builds, &err)
			if err != nil {
				return err
			}
			sort.Slice(builds, func(i int, j int) bool {
				return builds[i].CreatedAt.Before(builds[j].CreatedAt)
			})
			fmt.Println(formatter.Format(builds))
			return nil
		}),
	}
	cmd.Flags().BoolVar(&filterF.Untagged, "untagged", false, "If set, only untagged builds are listed")
	cmd.Flags().StringVar(&formatF.Formatter, "format", "table", "Name of formatter used to format the output: "+strings.Join(c.Names((*format.Formatter)(nil)), " | "))
	return cmd
}

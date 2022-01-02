package commands

import (
	"fmt"
	"sort"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/osman"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/format"
	"github.com/outofforest/osman/infra/types"
	"github.com/spf13/cobra"
)

// NewListCommand returns new list command
func NewListCommand(cmdF *CmdFactory) *cobra.Command {
	var loggingF *config.LoggingFactory
	var storageF *config.StorageFactory
	var filterF *config.FilterFactory
	var formatF *config.FormatFactory

	cmd := &cobra.Command{
		Short: "Lists information about available builds",
		Use:   "list [flags] [... buildID | [name][:tag]]",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
			c.Singleton(loggingF.Config)
			c.Singleton(storageF.Config)
			c.Singleton(filterF.Config)
			c.Singleton(formatF.Config)
		}, func(c *ioc.Container, formatter format.Formatter) error {
			var builds []types.BuildInfo
			var err error
			c.Call(osman.List, &builds, &err)
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

	loggingF = cmdF.AddLoggingFlags(cmd)
	storageF = cmdF.AddStorageFlags(cmd)
	filterF = cmdF.AddFilterFlags(cmd, []string{config.BuildTypeImage, config.BuildTypeMount})
	formatF = cmdF.AddFormatFlags(cmd)
	return cmd
}

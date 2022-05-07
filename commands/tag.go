package commands

import (
	"fmt"
	"sort"

	"github.com/outofforest/ioc/v2"
	"github.com/spf13/cobra"

	"github.com/outofforest/osman"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/format"
	"github.com/outofforest/osman/infra/types"
)

// NewTagCommand returns new tag command
func NewTagCommand(cmdF *CmdFactory) *cobra.Command {
	var loggingF *config.LoggingFactory
	var storageF *config.StorageFactory
	var filterF *config.FilterFactory
	var tagF *config.TagFactory
	var formatF *config.FormatFactory

	cmd := &cobra.Command{
		Short: "Removes and adds tags to the builds",
		Use:   "tag [flags] [... buildID | [name][:tag]]",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
			c.Singleton(loggingF.Config)
			c.Singleton(storageF.Config)
			c.Singleton(filterF.Config)
			c.Singleton(tagF.Config)
			c.Singleton(formatF.Config)
		}, func(c *ioc.Container, formatter format.Formatter) error {
			var builds []types.BuildInfo
			var err error
			c.Call(osman.Tag, &builds, &err)
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
	filterF = cmdF.AddFilterFlags(cmd, []string{config.BuildTypeImage})
	tagF = cmdF.AddTagFlags(cmd)
	formatF = cmdF.AddFormatFlags(cmd)
	return cmd
}

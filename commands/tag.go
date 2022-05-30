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
	var storageF *config.StorageFactory
	var filterF *config.FilterFactory
	var formatF *config.FormatFactory
	tagF := &config.TagFactory{}

	cmd := &cobra.Command{
		Short: "Removes and adds tags to the builds",
		Use:   "tag [flags] [... buildID | [name][:tag]]",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
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
	storageF = cmdF.AddStorageFlags(cmd)
	filterF = cmdF.AddFilterFlags(cmd, []string{config.BuildTypeImage})
	formatF = cmdF.AddFormatFlags(cmd)
	cmd.Flags().StringSliceVar(&tagF.Remove, "remove", []string{}, "Tag to be removed")
	cmd.Flags().StringSliceVar(&tagF.Add, "add", []string{}, "Tag to be added")
	cmd.Flags().BoolVar(&tagF.All, "all", false, "It is required to set this flag to tag builds if no filters are provided")
	return cmd
}

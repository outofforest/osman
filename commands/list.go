package commands

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/osman"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/format"
	"github.com/outofforest/osman/infra/types"
)

var defaultFields = []string{"BuildID", "BasedOn", "CreatedAt", "Name", "Tags", "Mounted"}

// NewListCommand returns new list command.
func NewListCommand(cmdF *CmdFactory) *cobra.Command {
	var storageF *config.StorageFactory
	var filterF *config.FilterFactory
	var formatF *config.FormatFactory

	cmd := &cobra.Command{
		Short: "Lists information about available builds",
		Use:   "list [flags] [... buildID | [name][:tag]]",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
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
			fmt.Println(formatter.Format(builds, defaultFields...))
			return nil
		}),
	}

	storageF = cmdF.AddStorageFlags(cmd)
	filterF = cmdF.AddFilterFlags(cmd, []string{config.BuildTypeImage, config.BuildTypeMount, config.BuildTypeBoot,
		config.BuildTypeVM})
	formatF = cmdF.AddFormatFlags(cmd)
	return cmd
}

package commands

import (
	"fmt"
	"os"

	"github.com/ridge/must"
	"github.com/spf13/cobra"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/osman"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/description"
	"github.com/outofforest/osman/infra/format"
	"github.com/outofforest/osman/infra/types"
)

// NewBuildCommand creates new build command.
func NewBuildCommand(cmdF *CmdFactory) *cobra.Command {
	var storageF *config.StorageFactory
	var formatF *config.FormatFactory
	buildF := &config.BuildFactory{}

	cmd := &cobra.Command{
		Short: "Builds images from spec files",
		Args:  cobra.MinimumNArgs(1),
		Use:   "build [flags] ...specfile",
		RunE: cmdF.Cmd(func(c *ioc.Container) {
			c.Singleton(storageF.Config)
			c.Singleton(formatF.Config)
			c.Singleton(buildF.Config)
		}, func(c *ioc.Container, formatter format.Formatter) error {
			var builds []types.BuildInfo
			var err error
			c.Call(osman.Build, &builds, &err)
			if err != nil {
				return err
			}
			fmt.Println(formatter.Format(builds, defaultFields...))
			return nil
		}),
	}
	storageF = cmdF.AddStorageFlags(cmd)
	formatF = cmdF.AddFormatFlags(cmd)
	cmd.Flags().StringSliceVar(&buildF.Names, "name", []string{},
		"Name of built image, if empty name is derived from corresponding specfile")
	cmd.Flags().StringSliceVar(&buildF.Tags, "tag", []string{string(description.DefaultTag)},
		"Tags assigned to created build")
	cmd.Flags().BoolVar(&buildF.Rebuild, "rebuild", false,
		"If set, all parent images are rebuilt even if they exist")
	cmd.Flags().StringVar(&buildF.CacheDir, "cache-dir", must.String(os.UserCacheDir())+"/osman",
		"Path to a directory where files are cached")
	return cmd
}

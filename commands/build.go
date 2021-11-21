package commands

import (
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder"
	"github.com/wojciech-malota-wojcik/imagebuilder/config"
)

// NewBuildCommand creates new build command
func NewBuildCommand(buildF *config.BuildFactory, cmdF *CmdFactory) *cobra.Command {
	cmd := &cobra.Command{
		Short: "Builds images from spec files",
		Args:  cobra.MinimumNArgs(1),
		Use:   "build [flags] ...specfile",
		RunE:  cmdF.Cmd(imagebuilder.Build),
	}
	cmd.Flags().StringSliceVar(&buildF.Names, "name", []string{}, "Name of built image, if empty name is derived from corresponding specfile")
	cmd.Flags().StringSliceVar(&buildF.Tags, "tag", []string{string(config.DefaultTag)}, "Tags assigned to created build")
	cmd.Flags().BoolVar(&buildF.Rebuild, "rebuild", false, "If set, all parent images are rebuilt even if they exist")
	return cmd
}

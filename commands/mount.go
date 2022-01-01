package commands

import (
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder"
)

// NewMountCommand creates new mount command
func NewMountCommand(cmdF *CmdFactory) *cobra.Command {
	cmd := &cobra.Command{
		Short: "Mounts image",
		Args:  cobra.MinimumNArgs(2),
		Use:   "mount [flags] image name",
		RunE:  cmdF.Cmd(imagebuilder.Mount),
	}
	return cmd
}

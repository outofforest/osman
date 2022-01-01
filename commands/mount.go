package commands

import (
	"github.com/outofforest/osman"
	"github.com/spf13/cobra"
)

// NewMountCommand creates new mount command
func NewMountCommand(cmdF *CmdFactory) *cobra.Command {
	cmd := &cobra.Command{
		Short: "Mounts image",
		Args:  cobra.MinimumNArgs(2),
		Use:   "mount [flags] image name",
		RunE:  cmdF.Cmd(osman.Mount),
	}
	return cmd
}

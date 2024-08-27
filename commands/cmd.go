package commands

import (
	"os"
	"strings"

	"github.com/ridge/must"
	"github.com/spf13/cobra"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/format"
	"github.com/outofforest/osman/infra/storage"
)

// NewCmdFactory returns new CmdFactory.
func NewCmdFactory(c *ioc.Container) *CmdFactory {
	return &CmdFactory{
		c: c,
	}
}

// CmdFactory is a wrapper around cobra RunE.
type CmdFactory struct {
	c *ioc.Container
}

// Cmd returns function compatible with RunE.
func (f *CmdFactory) Cmd(setupFunc interface{}, cmdFunc interface{}) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		f.c.Singleton(func() config.Args {
			return args
		})
		if setupFunc != nil {
			f.c.Call(setupFunc)
		}
		var err error
		f.c.Call(cmdFunc, &err)
		return err
	}
}

// AddStorageFlags adds storage flags to command.
func (f *CmdFactory) AddStorageFlags(cmd *cobra.Command) *config.StorageFactory {
	storageF := &config.StorageFactory{}

	cmd.Flags().StringVar(&storageF.Root, "storage-root", "tank/builds/"+must.String(os.Hostname()),
		"Location where built images are stored")
	cmd.Flags().StringVar(&storageF.Driver, "storage-driver", "zfs",
		"Storage driver to use: "+strings.Join(f.c.Names((*storage.Driver)(nil)), " | "))

	return storageF
}

// AddFilterFlags adds filtering flags to command.
func (f *CmdFactory) AddFilterFlags(cmd *cobra.Command, defaultTypes []string) *config.FilterFactory {
	filterF := &config.FilterFactory{}

	cmd.Flags().StringSliceVar(&filterF.Types, "type", defaultTypes,
		"Consider only builds of specified types: "+strings.Join(config.BuildTypes(), " | "))
	cmd.Flags().BoolVar(&filterF.Untagged, "untagged", false,
		"If set, only untagged builds are considered")

	return filterF
}

// AddFormatFlags adds formatting flags to command.
func (f *CmdFactory) AddFormatFlags(cmd *cobra.Command) *config.FormatFactory {
	formatF := &config.FormatFactory{}

	cmd.Flags().StringVar(&formatF.Formatter, "format", "table",
		"Name of formatter used to format the output: "+strings.Join(f.c.Names((*format.Formatter)(nil)), " | "))

	return formatF
}

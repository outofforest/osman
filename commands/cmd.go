package commands

import (
	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/logger"
	"github.com/outofforest/osman/config"
	"github.com/spf13/cobra"
)

// NewCmdFactory returns new CmdFactory
func NewCmdFactory(c *ioc.Container) *CmdFactory {
	return &CmdFactory{
		c: c,
	}
}

// CmdFactory is a wrapper around cobra RunE
type CmdFactory struct {
	c *ioc.Container
}

// Cmd returns function compatible with RunE
func (f *CmdFactory) Cmd(cmdFunc interface{}) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		f.c.Singleton(func() config.Args {
			return args
		})
		var err error
		f.c.Resolve(func(c *ioc.Container, configRoot config.Root) {
			if !configRoot.VerboseLogging {
				logger.VerboseOff()
			}
			f.c.Call(cmdFunc, &err)
		})
		return err
	}
}

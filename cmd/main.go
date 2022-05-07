package main

import (
	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/run"
	"github.com/spf13/cobra"

	"github.com/outofforest/osman/commands"
	"github.com/outofforest/osman/infra"
	"github.com/outofforest/osman/infra/base"
	"github.com/outofforest/osman/infra/format"
	"github.com/outofforest/osman/infra/parser"
	"github.com/outofforest/osman/infra/storage"
)

func iocBuilder(c *ioc.Container) {
	c.Singleton(commands.NewCmdFactory)
	c.Singleton(base.NewDockerInitializer)
	c.Singleton(infra.NewRepository)
	c.Transient(infra.NewBuilder)

	c.Singleton(storage.Resolve)
	c.SingletonNamed("dir", storage.NewDirDriver)
	c.SingletonNamed("zfs", storage.NewZFSDriver)

	c.Singleton(parser.NewResolvingParser)
	c.SingletonNamed("spec", parser.NewSpecFileParser)

	c.Singleton(format.Resolve)
	c.SingletonNamed("table", format.NewTableFormatter)
	c.SingletonNamed("json", format.NewJSONFormatter)

	c.Singleton(commands.NewRootCommand)
	c.SingletonNamed("build", commands.NewBuildCommand)
	c.SingletonNamed("mount", commands.NewMountCommand)
	c.SingletonNamed("list", commands.NewListCommand)
	c.SingletonNamed("drop", commands.NewDropCommand)
	c.SingletonNamed("tag", commands.NewTagCommand)
}

func main() {
	run.Tool("osman", iocBuilder, func(rootCmd *cobra.Command) error {
		return rootCmd.Execute()
	})
}

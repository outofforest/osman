package main

import (
	"context"

	"github.com/outofforest/osman/commands"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra"
	"github.com/outofforest/osman/infra/base"
	"github.com/outofforest/osman/infra/format"
	"github.com/outofforest/osman/infra/parser"
	"github.com/outofforest/osman/infra/storage"
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/ioc/v2"
	"github.com/wojciech-malota-wojcik/run"
)

func iocBuilder(c *ioc.Container) {
	c.Singleton(commands.NewCmdFactory)
	c.Singleton(base.NewDockerInitializer)
	c.Singleton(infra.NewRepository)
	c.Transient(infra.NewBuilder)

	c.Singleton(config.NewRootFactory)
	c.Singleton(config.NewFilterFactory)
	c.Singleton(config.NewStorageFactory)
	c.Singleton(config.NewFormatFactory)
	c.Singleton(config.NewBuildFactory)
	c.Singleton(config.NewDropFactory)
	c.Singleton(config.NewRoot)
	c.Singleton(config.NewFilter)
	c.Singleton(config.NewStorage)
	c.Singleton(config.NewFormat)
	c.Singleton(config.NewBuild)
	c.Singleton(config.NewMount)
	c.Singleton(config.NewDrop)

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
}

func main() {
	run.Tool("imagebuilder", iocBuilder, func(ctx context.Context, rootCmd *cobra.Command) error {
		return rootCmd.Execute()
	})
}

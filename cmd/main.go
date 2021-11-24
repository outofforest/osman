package main

import (
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands"
	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/format"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/parser"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/ioc"
	"github.com/wojciech-malota-wojcik/run"
)

func iocBuilder(c *ioc.Container) {
	c.Singleton(commands.NewCmdFactory)
	c.Singleton(storage.NewDirDriver)
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
	c.Singleton(config.NewDrop)

	c.Singleton(parser.NewResolvingParser)
	c.SingletonNamed("spec", parser.NewSpecFileParser)

	c.Singleton(format.Resolve)
	c.SingletonNamed("table", format.NewTableFormatter)
	c.SingletonNamed("json", format.NewJSONFormatter)

	c.Singleton(commands.NewRootCommand)
	c.SingletonNamed("build", commands.NewBuildCommand)
	c.SingletonNamed("list", commands.NewListCommand)
	c.SingletonNamed("drop", commands.NewDropCommand)
}

func main() {
	run.Tool("imagebuilder", iocBuilder, func(rootCmd *cobra.Command) error {
		return rootCmd.Execute()
	})
}

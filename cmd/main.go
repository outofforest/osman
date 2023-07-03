package main

import (
	"context"

	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/isolator/executor"
	"github.com/outofforest/isolator/wire"
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
	c.SingletonNamed("zfs", storage.NewZFSDriver)

	c.Singleton(parser.NewResolvingParser)
	c.SingletonNamed("spec", parser.NewSpecFileParser)

	c.Singleton(format.Resolve)
	c.SingletonNamed("table", format.NewTableFormatter)
	c.SingletonNamed("json", format.NewJSONFormatter)

	c.Singleton(commands.NewRootCommand)
	c.SingletonNamed("build", commands.NewBuildCommand)
	c.SingletonNamed("mount", commands.NewMountCommand)
	c.SingletonNamed("start", commands.NewStartCommand)
	c.SingletonNamed("stop", commands.NewStopCommand)
	c.SingletonNamed("list", commands.NewListCommand)
	c.SingletonNamed("drop", commands.NewDropCommand)
	c.SingletonNamed("tag", commands.NewTagCommand)
}

func main() {
	run.New().
		WithContainerBuilder(iocBuilder).
		WithFlavour(executor.NewFlavour(executor.Config{
			Router: executor.NewRouter().
				RegisterHandler(wire.Execute{}, executor.ExecuteHandler).
				RegisterHandler(wire.InitFromDocker{}, executor.NewInitFromDockerHandler()),
		})).
		Run("osman", func(ctx context.Context, rootCmd *cobra.Command) error {
			return rootCmd.Execute()
		})
}

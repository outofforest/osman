package main

import (
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/build"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/list"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/root"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/format"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/ioc"
	"github.com/wojciech-malota-wojcik/run"
)

func iocBuilder(c *ioc.Container) {
	c.Singleton(commands.NewCmdFactory)
	c.Singleton(storage.NewDirDriver)
	c.Singleton(infra.NewRepository)
	c.Transient(infra.NewBuilder)

	c.Singleton(format.Resolve)
	c.SingletonNamed("json", format.NewJSONFormatter)

	root.Install(c)
	build.Install(c)
	list.Install(c)
}

func main() {
	run.Tool("imagebuilder", iocBuilder, func(rootCmd *cobra.Command) error {
		return rootCmd.Execute()
	})
}

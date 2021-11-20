package imagebuilder

import (
	"github.com/wojciech-malota-wojcik/imagebuilder/commands"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/build"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/list"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/root"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/ioc"
)

// IoCBuilder configures ioc container
func IoCBuilder(c *ioc.Container) {
	c.Singleton(commands.NewCmdFactory)
	c.Singleton(storage.NewDirDriver)
	c.Singleton(infra.NewRepository)
	c.Transient(infra.NewBuilder)

	root.Install(c)
	build.Install(c)
	list.Install(c)
}

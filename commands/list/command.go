package list

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/list/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/format"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/ioc"
)

// Install installs list command
func Install(c *ioc.Container) {
	c.Singleton(func() *configFactory {
		return &configFactory{}
	})
	c.Singleton(newConfig)
	c.TransientNamed("list", command)
}

type configFactory struct {
	// Formatter is the name of formatter to use to convert list into string
	Formatter string
}

func newConfig(cf *configFactory, args commands.Args) config.List {
	config := config.List{
		Formatter: cf.Formatter,
		BuildIDs:  make([]types.BuildID, 0, len(args)),
		BuildKeys: make([]types.BuildKey, 0, len(args)),
	}

	for _, arg := range args {
		buildID, err := types.ParseBuildID(arg)
		if err == nil {
			config.BuildIDs = append(config.BuildIDs, buildID)
			continue
		}

		buildKey, err := types.ParseBuildKey(arg)
		if err != nil {
			panic(fmt.Errorf("argument '%s' is neither valid build ID nor build key", arg))
		}
		config.BuildKeys = append(config.BuildKeys, buildKey)
	}
	return config
}

func command(c *ioc.Container, cf *configFactory, cmdF *commands.CmdFactory) *cobra.Command {
	cmd := &cobra.Command{
		Short: "List information about available builds",
		Use:   "list [... buildID | [name][:tag]]",
		RunE: cmdF.Cmd(func(c *ioc.Container, formatter format.Formatter) error {
			var builds []storage.BuildInfo
			var err error
			c.Call(imagebuilder.List, &builds, &err)
			if err != nil {
				return err
			}
			fmt.Println(formatter.Format(builds))
			return nil
		}),
	}
	cmd.Flags().StringVar(&cf.Formatter, "format", "table", "Name of formatter used to format the output: "+strings.Join(c.Names((*format.Formatter)(nil)), " | "))
	return cmd
}

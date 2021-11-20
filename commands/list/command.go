package list

import (
	"encoding/json"
	"fmt"

	"github.com/ridge/must"
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/list/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/ioc"
)

// Install installs list command
func Install(c *ioc.Container) {
	c.Singleton(newConfig)
	c.TransientNamed("list", command)
}

func newConfig(args commands.Args) config.List {
	config := config.List{
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

func command(cmdF *commands.CmdFactory) *cobra.Command {
	return &cobra.Command{
		Short: "List information about available builds",
		Use:   "list [...buildID]",
		RunE: cmdF.Cmd(func(c *ioc.Container) error {
			var list []storage.BuildInfo
			var err error
			c.Call(imagebuilder.List, &list, &err)
			if err != nil {
				return err
			}
			fmt.Println(string(must.Bytes(json.MarshalIndent(list, "", "  "))))
			return nil
		}),
	}
}

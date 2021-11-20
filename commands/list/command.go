package list

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ridge/must"
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands"
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/list/config"
	configRoot "github.com/wojciech-malota-wojcik/imagebuilder/commands/root/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/ioc"
)

// Install installs list command
func Install(c *ioc.Container) {
	c.Singleton(newConfig)
	c.TransientNamed("list", command)
}

func newConfig(configRoot configRoot.Root, args commands.Args) config.List {
	config := config.List{
		Root:     configRoot,
		BuildIDs: make([]types.BuildID, 0, len(args)),
	}

	for _, arg := range args {
		buildID, err := types.ParseBuildID(arg)
		if err != nil {
			panic(err)
		}
		config.BuildIDs = append(config.BuildIDs, buildID)
	}
	return config
}

func command(cmdF *commands.CmdFactory) *cobra.Command {
	return &cobra.Command{
		Short: "List information about available builds",
		Use:   "list [...buildID]",
		RunE: cmdF.Cmd(func(config config.List, s storage.Driver) error {
			var buildIDs map[types.BuildID]bool
			if len(config.BuildIDs) > 0 {
				buildIDs = map[types.BuildID]bool{}
				for _, buildID := range config.BuildIDs {
					buildIDs[buildID] = true
				}
			}

			builds, err := s.Builds()
			if err != nil {
				return err
			}
			res := make([]storage.BuildInfo, 0, len(builds))
			for _, buildID := range builds {
				if buildIDs != nil && !buildIDs[buildID] {
					continue
				}
				info, err := s.Info(buildID)
				if err != nil {
					return err
				}
				sort.Sort(types.TagSlice(info.Tags))
				res = append(res, info)
			}

			sort.Slice(res, func(i int, j int) bool {
				return res[i].CreatedAt.Before(res[j].CreatedAt)
			})

			fmt.Println(string(must.Bytes(json.MarshalIndent(res, "", "  "))))
			return nil
		}),
	}
}

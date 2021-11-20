package list

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ridge/must"
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/imagebuilder/runtime"
	"github.com/wojciech-malota-wojcik/ioc"
)

// Install installs list command
func Install(c *ioc.Container) {
	c.TransientNamed("list", command)
}

func command(cmdF *runtime.CmdFactory) *cobra.Command {
	return &cobra.Command{
		Short: "List information about available builds",
		Use:   "list",
		RunE: cmdF.Cmd(func(s storage.Driver) error {
			builds, err := s.Builds()
			if err != nil {
				return err
			}
			res := make([]storage.BuildInfo, 0, len(builds))
			for _, buildID := range builds {
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

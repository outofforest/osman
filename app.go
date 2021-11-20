package imagebuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ridge/must"
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/runtime"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/ioc"
	"github.com/wojciech-malota-wojcik/logger"
	"go.uber.org/zap"
)

// IoCBuilder configures ioc container
func IoCBuilder(c *ioc.Container) {
	c.Singleton(runtime.NewConfigRootFactory)
	c.Singleton(runtime.NewConfigBuildFactory)
	c.Singleton(runtime.NewConfigRoot)
	c.Singleton(runtime.NewConfigBuild)
	c.Singleton(runtime.NewCmdFactory)
	c.Singleton(storage.NewDirDriver)
	c.Singleton(infra.NewRepository)
	c.Transient(infra.NewBuilder)
}

// App runs builder app
func App(configRootF *runtime.ConfigRootFactory, configBuildF *runtime.ConfigBuildFactory, cmdF *runtime.CmdFactory) error {
	rootCmd := &cobra.Command{
		SilenceUsage: true,
	}
	rootCmd.PersistentFlags().StringVar(&configRootF.RootDir, "root-dir", filepath.Join(must.String(os.UserHomeDir()), ".images"), "Directory where built images are stored")
	rootCmd.PersistentFlags().BoolVarP(&configRootF.VerboseLogging, "verbose", "v", false, "Turns on verbose logging")

	listCmd := &cobra.Command{
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

	buildCmd := &cobra.Command{
		Short: "Builds images from spec files",
		Args:  cobra.MinimumNArgs(1),
		Use:   "build [flags] ...specfile",
		RunE: cmdF.Cmd(func(ctx context.Context, config runtime.ConfigBuild, repo *infra.Repository, builder *infra.Builder) error {
			fedoraCmds := []infra.Command{infra.Run(`printf "nameserver 8.8.8.8\nnameserver 8.8.4.4\n" > /etc/resolv.conf`),
				infra.Run(`echo 'LANG="en_US.UTF-8"' > /etc/locale.conf`),
				infra.Run(`rm -rf /var/cache/* /tmp/*`)}

			repo.Store(infra.Describe("fedora", []types.Tag{"34"}, fedoraCmds...))
			repo.Store(infra.Describe("fedora", []types.Tag{"35"}, fedoraCmds...))

			for i, specFile := range config.SpecFiles {
				must.OK(os.Chdir(filepath.Dir(specFile)))

				build, err := builder.BuildFromFile(ctx, specFile, config.Names[i], config.Tags...)
				if err != nil {
					return err
				}
				logger.Get(ctx).Info("Image built", zap.Strings("params", build.Manifest().Params))
			}
			return nil
		}),
	}
	buildCmd.Flags().StringSliceVar(&configBuildF.Names, "name", []string{}, "Name of built image, if empty name is derived from corresponding specfile")
	buildCmd.Flags().StringSliceVar(&configBuildF.Tags, "tag", []string{string(runtime.DefaultTag)}, "Tags assigned to created build")
	buildCmd.Flags().BoolVar(&configBuildF.Rebuild, "rebuild", false, "If set, all parent images are rebuilt even if they exist")

	rootCmd.AddCommand(listCmd, buildCmd)
	return rootCmd.Execute()
}

package imagebuilder

import (
	"context"
	"os"
	"path/filepath"

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
	c.Singleton(runtime.NewConfigFactory)
	c.Singleton(runtime.NewConfigFromFactory)
	c.Singleton(runtime.NewCmdFactory)
	c.Singleton(storage.NewDirDriver)
	c.Singleton(infra.NewRepository)
	c.Transient(infra.NewBuilder)
}

// App runs builder app
func App(cf *runtime.ConfigFactory, cmdF *runtime.CmdFactory) error {
	rootCmd := &cobra.Command{
		SilenceUsage: true,
	}
	rootCmd.PersistentFlags().StringVar(&cf.RootDir, "root-dir", filepath.Join(must.String(os.UserHomeDir()), ".images"), "Directory where built images are stored")
	rootCmd.PersistentFlags().BoolVarP(&cf.VerboseLogging, "verbose", "v", false, "Turns on verbose logging")

	buildCmd := &cobra.Command{
		Short: "Builds images from spec files",
		Args:  cobra.MinimumNArgs(1),
		Use:   "build [flags] ...specfile",
		RunE: cmdF.Cmd(&cf.SpecFiles, func(ctx context.Context, config runtime.Config, repo *infra.Repository, builder *infra.Builder) error {
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
	buildCmd.Flags().StringSliceVar(&cf.Names, "name", []string{}, "Name of built image, if empty name is derived from corresponding specfile")
	buildCmd.Flags().StringSliceVar(&cf.Tags, "tag", []string{string(runtime.DefaultTag)}, "Tags assigned to created build")
	buildCmd.Flags().BoolVar(&cf.Rebuild, "rebuild", false, "If set, all parent images are rebuilt even if they exist")

	rootCmd.AddCommand(buildCmd)
	return rootCmd.Execute()
}

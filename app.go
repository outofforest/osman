package imagebuilder

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ridge/must"
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
	c.Singleton(runtime.NewConfigFromCLI)
	c.Singleton(storage.NewDirDriver)
	c.Singleton(infra.NewRepository)
	c.Transient(infra.NewBuilder)
}

// App runs builder app
func App(ctx context.Context, config runtime.Config, repo *infra.Repository, builder *infra.Builder) error {
	if !config.VerboseLogging {
		logger.VerboseOff()
	}

	fedoraCmds := []infra.Command{infra.Run(`printf "nameserver 8.8.8.8\nnameserver 8.8.4.4\n" > /etc/resolv.conf`),
		infra.Run(`echo 'LANG="en_US.UTF-8"' > /etc/locale.conf`),
		infra.Run(`rm -rf /var/cache/* /tmp/*`)}

	repo.Store(infra.Describe("fedora", []types.Tag{"34"}, fedoraCmds...))
	repo.Store(infra.Describe("fedora", []types.Tag{"35"}, fedoraCmds...))

	for i, specFile := range config.SpecFiles {
		must.OK(os.Chdir(filepath.Dir(specFile)))

		build, err := infra.BuildFromFile(ctx, builder, specFile, config.Names[i], config.Tags...)
		if err != nil {
			return err
		}
		logger.Get(ctx).Info("Image built", zap.Strings("params", build.Manifest().Params))
	}

	return nil
}

package imagebuilder

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/ridge/must"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/parse"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/runtime"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
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

	must.OK(os.Chdir(filepath.Dir(config.Specfile)))

	repo.Store(infra.Describe("fedora:34",
		infra.Params("param1"),
		infra.Run(`printf "nameserver 8.8.8.8\nnameserver 8.8.4.4\n" > /etc/resolv.conf`),
		infra.Run(`echo 'LANG="en_US.UTF-8"' > /etc/locale.conf`)))

	commands, err := parse.Parse(config.Specfile)
	if err != nil {
		return err
	}
	img := infra.Describe(strings.TrimSuffix(filepath.Base(config.Specfile), ".spec"), commands...)
	build, err := builder.Build(ctx, img)
	if err != nil {
		return err
	}

	logger.Get(ctx).Info("Image built", zap.String("path", build.Path()), zap.Strings("params", build.Manifest().Params))

	return nil
}

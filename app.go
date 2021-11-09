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
	c.Singleton(func() runtime.Config {
		return runtime.Config{
			RootDir:    "/tmp/images",
			Dockerfile: "/home/wojciech/sources/imagebuilder/base.image",
		}
	})
	c.Singleton(storage.NewDirDriver)
	c.Singleton(infra.NewRepository)
	c.Transient(infra.NewBuilder)
}

// App runs builder app
func App(ctx context.Context, config runtime.Config, repo *infra.Repository, builder *infra.Builder) error {
	must.OK(os.Chdir(filepath.Dir(config.Dockerfile)))

	commands, err := parse.Parse(config.Dockerfile)
	if err != nil {
		return err
	}
	img := infra.Describe(strings.TrimSuffix(filepath.Base(config.Dockerfile), ".image"), commands...)
	build, err := builder.Build(ctx, img)
	if err != nil {
		return err
	}

	logger.Get(ctx).Info("Image built", zap.String("path", build.Path()),
		zap.String("label", build.Label("pl.woojciki.wojciech.kernel-param.blacklist-nouveau")))

	return nil
}

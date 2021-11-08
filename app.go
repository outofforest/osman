package imagebuilder

import (
	"context"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra"
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
			RootDir: "/tmp/images",
		}
	})
	c.Singleton(storage.NewDirDriver)
	c.Singleton(infra.NewRepository)
	c.Transient(infra.NewBuilder)
}

// App runs builder app
func App(ctx context.Context, repo *infra.Repository, builder *infra.Builder) error {
	img := infra.Describe("base",
		infra.FromScratch(),
		infra.Label("test", "testValue"),
		infra.Run("dnf -y install kernel"))
	build, err := builder.Build(ctx, img)
	if err != nil {
		return err
	}

	logger.Get(ctx).Info("Image built", zap.String("path", build.Path()))

	return nil
}

package base

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/outofforest/isolator"
	"github.com/outofforest/isolator/wire"
	"github.com/outofforest/osman/infra/types"
)

// NewDockerInitializer creates new initializer getting base images from docker registry.
func NewDockerInitializer() Initializer {
	return &dockerInitializer{}
}

type dockerInitializer struct {
}

// Init fetches image from docker registry and integrates it inside directory.
func (f *dockerInitializer) Init(ctx context.Context, cacheDir, dir string, buildKey types.BuildKey) (retErr error) {
	cacheDir = filepath.Join(cacheDir, "docker-images")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return errors.WithStack(err)
	}

	return isolator.Run(ctx, isolator.Config{
		Dir: dir,
		Types: []interface{}{
			wire.Result{},
		},
		Executor: wire.Config{
			UseHostNetwork: true,
			Mounts: []wire.Mount{
				{
					Host:      cacheDir,
					Namespace: "/.docker-cache",
					Writable:  true,
				},
			},
		},
	}, func(ctx context.Context, incoming <-chan interface{}, outgoing chan<- interface{}) error {
		select {
		case <-ctx.Done():
			return errors.WithStack(ctx.Err())
		case outgoing <- wire.InflateDockerImage{
			CacheDir: "/.docker-cache",
			Image:    buildKey.Name,
			Tag:      string(buildKey.Tag),
		}:
		}

		for content := range incoming {
			switch m := content.(type) {
			case wire.Result:
				if m.Error != "" {
					return errors.New(m.Error)
				}
				return nil
			default:
				return errors.New("unexpected message received")
			}
		}

		return errors.WithStack(ctx.Err())
	})
}

package base

import (
	"context"
	"os"
	"path/filepath"

	"github.com/outofforest/isolator"
	"github.com/outofforest/isolator/wire"
	"github.com/outofforest/parallel"
	"github.com/pkg/errors"

	"github.com/outofforest/osman/infra/types"
)

// NewDockerInitializer creates new initializer getting base images from docker registry
func NewDockerInitializer() Initializer {
	return &dockerInitializer{}
}

type dockerInitializer struct {
}

// Init fetches image from docker registry and integrates it inside directory
func (f *dockerInitializer) Init(ctx context.Context, cacheDir, dir string, buildKey types.BuildKey) error {
	return parallel.Run(ctx, func(ctx context.Context, spawn parallel.SpawnFn) error {
		incoming := make(chan interface{})
		outgoing := make(chan interface{})

		cacheDir := filepath.Join(cacheDir, "docker-images")
		if err := os.MkdirAll(cacheDir, 0o700); err != nil {
			return errors.WithStack(err)
		}

		spawn("isolator", parallel.Fail, func(ctx context.Context) error {
			return isolator.Run(ctx, isolator.Config{
				Dir: dir,
				Types: []interface{}{
					wire.Result{},
				},
				Executor: wire.Config{
					NoStandardMounts: true,
					Mounts: []wire.Mount{
						{
							Host:      cacheDir,
							Container: "/.docker-cache",
							Writable:  true,
						},
					},
				},
				Incoming: incoming,
				Outgoing: outgoing,
			})
		})
		spawn("init", parallel.Exit, func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return errors.WithStack(ctx.Err())
			case outgoing <- wire.InflateDockerImage{
				CacheDir: "/.docker-cache",
				Image:    buildKey.Name,
				Tag:      string(buildKey.Tag),
			}:
			}

			select {
			case <-ctx.Done():
				return errors.WithStack(ctx.Err())
			case content := <-incoming:
				result, ok := content.(wire.Result)
				if !ok {
					return errors.Errorf("expected Result, got: %T", content)
				}
				if result.Error != "" {
					return errors.New(result.Error)
				}
				return nil
			}
		})
		return nil
	})
}

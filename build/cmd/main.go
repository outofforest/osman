package main

import (
	"context"

	"github.com/outofforest/build"
	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/run"

	me "build"
)

func main() {
	run.Tool("build", nil, func(ctx context.Context, c *ioc.Container) error {
		return build.Do(ctx, "go-env-v1", build.NewIoCExecutor(me.Commands, c))
	})
}

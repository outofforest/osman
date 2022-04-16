package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/outofforest/build"
	"github.com/outofforest/ioc/v2"
	"github.com/outofforest/run"
	"github.com/ridge/must"

	me "github.com/outofforest/osman/build"
)

func main() {
	run.Tool("build", nil, func(ctx context.Context, c *ioc.Container) error {
		exec := build.NewIoCExecutor(me.Commands, c)
		if build.Autocomplete(exec) {
			return nil
		}

		changeWorkingDir()
		return build.Do(ctx, "go", exec)
	})
}

func changeWorkingDir() {
	must.OK(os.Chdir(filepath.Dir(filepath.Dir(must.String(filepath.EvalSymlinks(must.String(os.Executable())))))))
}

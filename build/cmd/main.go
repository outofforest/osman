package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ridge/must"
	"github.com/wojciech-malota-wojcik/build"
	me "github.com/wojciech-malota-wojcik/imagebuilder/build"
	"github.com/wojciech-malota-wojcik/ioc/v2"
	"github.com/wojciech-malota-wojcik/run"
)

func main() {
	run.Tool("build", nil, func(ctx context.Context, c *ioc.Container) error {
		exec := build.NewIoCExecutor(me.Commands, c)
		if build.Autocomplete(exec) {
			return nil
		}

		changeWorkingDir()
		return build.Do(ctx, "Digest", exec)
	})
}

func changeWorkingDir() {
	must.OK(os.Chdir(filepath.Dir(filepath.Dir(must.String(filepath.EvalSymlinks(must.String(os.Executable())))))))
}

package build

import (
	"context"

	"github.com/outofforest/build"
	"github.com/outofforest/buildgo"
)

func setup(deps build.DepsFunc) {
	deps(buildgo.InstallAll)
}

func buildMe(ctx context.Context, deps build.DepsFunc) error {
	deps(buildgo.EnsureGo)
	return buildgo.GoBuildPkg(ctx, "build/cmd", "bin/tmp-osman", true)
}

package build

import (
	"context"
	"os/exec"

	"github.com/wojciech-malota-wojcik/build"
	"github.com/wojciech-malota-wojcik/buildgo"
	"github.com/wojciech-malota-wojcik/libexec"
)

func buildApp(ctx context.Context) error {
	return buildgo.GoBuildPkg(ctx, "cmd", "bin/osman-app", false)
}

func runApp(ctx context.Context, deps build.DepsFunc) error {
	deps(buildApp)
	return libexec.Exec(ctx, exec.Command("./bin/osman-app"))
}

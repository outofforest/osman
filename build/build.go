package build

import (
	"context"

	"github.com/wojciech-malota-wojcik/buildgo"
)

func buildMe(ctx context.Context) error {
	return buildgo.GoBuildPkg(ctx, "build/cmd", "bin/tmp-digest", true)
}

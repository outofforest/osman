package build

import (
	"context"

	"github.com/outofforest/buildgo"
)

func buildMe(ctx context.Context) error {
	return buildgo.GoBuildPkg(ctx, "build/cmd", "bin/tmp-osman", true)
}

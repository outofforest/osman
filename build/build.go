package build

import (
	"github.com/outofforest/build"
	"github.com/outofforest/buildgo"
)

func setup(deps build.DepsFunc) {
	deps(buildgo.InstallAll)
}

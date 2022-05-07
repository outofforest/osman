package main

import (
	"github.com/outofforest/build"

	me "build"
)

func main() {
	build.Main("go-env-v1", nil, me.Commands)
}

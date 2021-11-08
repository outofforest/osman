package main

import (
	"github.com/wojciech-malota-wojcik/imagebuilder"
	"github.com/wojciech-malota-wojcik/run"
)

func main() {
	run.Service("imagebuilder", imagebuilder.IoCBuilder, imagebuilder.App)
}

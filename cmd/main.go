package main

import (
	"github.com/spf13/cobra"
	"github.com/wojciech-malota-wojcik/imagebuilder"
	"github.com/wojciech-malota-wojcik/run"
)

func main() {
	run.Tool("imagebuilder", imagebuilder.IoCBuilder, func(rootCmd *cobra.Command) error {
		return rootCmd.Execute()
	})
}

package build

import "github.com/outofforest/buildgo"

// Commands is a definition of commands available in build system
var Commands = map[string]interface{}{
	"setup":       setup,
	"tools/build": buildMe,
	"build":       buildApp,
	"run":         runApp,
}

func init() {
	buildgo.AddCommands(Commands)
}

package description

import (
	"github.com/outofforest/osman/infra/types"
)

// From returns handler for FROM command
func From(buildKey types.BuildKey) Command {
	if buildKey.Tag == "" {
		buildKey.Tag = DefaultTag
	}
	return &FromCommand{
		BuildKey: buildKey,
	}
}

// Params returns handler for PARAMS command
func Params(params ...string) Command {
	return &ParamsCommand{
		Params: params,
	}
}

// Run returns handler for RUN command
func Run(command string) Command {
	return &RunCommand{
		Command: command,
	}
}

// FromCommand executes FROM command
type FromCommand struct {
	BuildKey types.BuildKey
}

// Execute executes build command
func (cmd *FromCommand) Execute(build ImageBuild) error {
	return build.From(cmd)
}

// ParamsCommand executes PARAMS command
type ParamsCommand struct {
	Params []string
}

// Execute executes build command
func (cmd *ParamsCommand) Execute(build ImageBuild) error {
	return build.Params(cmd)
}

// RunCommand executes RUN command
type RunCommand struct {
	Command string
}

// Execute executes build command
func (cmd *RunCommand) Execute(build ImageBuild) error {
	return build.Run(cmd)
}

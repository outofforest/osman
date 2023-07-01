package description

import (
	"context"

	"github.com/outofforest/osman/infra/types"
)

var (
	_ Command = &FromCommand{}
	_ Command = &ParamsCommand{}
	_ Command = &RunCommand{}
	_ Command = &BootCommand{}
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

// Boot returns handler for BOOT command
func Boot(title string, params []string) Command {
	return &BootCommand{
		Title:  title,
		Params: params,
	}
}

// FromCommand executes FROM command
type FromCommand struct {
	BuildKey types.BuildKey
}

// Execute executes build command
func (cmd *FromCommand) Execute(ctx context.Context, build ImageBuild) error {
	return build.From(cmd)
}

// ParamsCommand executes PARAMS command
type ParamsCommand struct {
	Params []string
}

// Execute executes build command
func (cmd *ParamsCommand) Execute(ctx context.Context, build ImageBuild) error {
	return build.Params(cmd)
}

// RunCommand executes RUN command
type RunCommand struct {
	Command string
}

// Execute executes build command
func (cmd *RunCommand) Execute(ctx context.Context, build ImageBuild) error {
	return build.Run(ctx, cmd)
}

// BootCommand executes BOOT command
type BootCommand struct {
	Title  string
	Params []string
}

// Execute executes build command
func (cmd *BootCommand) Execute(ctx context.Context, build ImageBuild) error {
	return build.Boot(cmd)
}

package infra

import (
	"context"

	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// Command is implemented by commands available in SpecFile
type Command interface {
	execute(ctx context.Context, build *ImageBuild) error
}

// From returns handler for FROM command
func From(buildKey types.BuildKey) Command {
	if buildKey.Tag == "" {
		buildKey.Tag = config.DefaultTag
	}
	return &fromCommand{
		buildKey: buildKey,
	}
}

// Params returns handler for PARAMS command
func Params(params ...string) Command {
	return &paramsCommand{
		params: params,
	}
}

// Run returns handler for RUN command
func Run(command string) Command {
	return &runCommand{
		command: command,
	}
}

type fromCommand struct {
	buildKey types.BuildKey
}

func (cmd *fromCommand) execute(ctx context.Context, build *ImageBuild) error {
	return build.from(cmd)
}

type paramsCommand struct {
	params []string
}

func (cmd *paramsCommand) execute(ctx context.Context, build *ImageBuild) error {
	return build.params(cmd)
}

type runCommand struct {
	command string
}

func (cmd *runCommand) execute(ctx context.Context, build *ImageBuild) error {
	return build.run(ctx, cmd)
}

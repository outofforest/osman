package infra

import (
	"context"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// Command is implemented by commands available in Specfile
type Command interface {
	execute(ctx context.Context, build *ImageBuild) error
}

// From returns handler for FROM command
func From(imageName string, tag types.Tag) Command {
	return &fromCommand{
		imageName: imageName,
		tag:       tag,
	}
}

// Params returns handler for PARAMS command
func Params(params ...string) Command {
	return &paramsCommand{
		params: params,
	}
}

// Copy returns handler for COPY command
func Copy(from, to string) Command {
	return &copyCommand{
		from: from,
		to:   to,
	}
}

// Run returns handler for RUN command
func Run(command string) Command {
	return &runCommand{
		command: command,
	}
}

type fromCommand struct {
	imageName string
	tag       types.Tag
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

type copyCommand struct {
	from string
	to   string
}

func (cmd *copyCommand) execute(ctx context.Context, build *ImageBuild) error {
	return build.copy(cmd)
}

type runCommand struct {
	command string
}

func (cmd *runCommand) execute(ctx context.Context, build *ImageBuild) error {
	return build.run(ctx, cmd)
}

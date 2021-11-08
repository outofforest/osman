package infra

import "context"

// Command is implemented by commands available in Dockerfile
type Command interface {
	execute(ctx context.Context, build *ImageBuild) error
}

// From returns handler for FROM command
func From(imageName string) Command {
	return &fromCommand{
		imageName: imageName,
	}
}

// FromScratch returns handler for FROM command which creates image from scratch
func FromScratch() Command {
	return &fromCommand{
		imageName: fromScratch,
	}
}

// Label returns handler for LABEL command
func Label(name, value string) Command {
	return &labelCommand{
		name:  name,
		value: value,
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

// Include returns handler for INCLUDE command
func Include(file string) Command {
	return &includeCommand{
		file: file,
	}
}

type fromCommand struct {
	imageName string
}

func (cmd *fromCommand) execute(ctx context.Context, build *ImageBuild) error {
	return build.from(cmd)
}

type labelCommand struct {
	name  string
	value string
}

func (cmd *labelCommand) execute(ctx context.Context, build *ImageBuild) error {
	return build.label(cmd)
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

type includeCommand struct {
	file string
}

func (cmd *includeCommand) execute(ctx context.Context, build *ImageBuild) error {
	return build.include(cmd)
}

package description

import (
	"context"

	"github.com/outofforest/osman/infra/types"
)

// DefaultTag is used if user specified empty tag list
const DefaultTag types.Tag = "latest"

// Command is implemented by commands available in SpecFile
type Command interface {
	// Execute executes build command
	Execute(ctx context.Context, build ImageBuild) error
}

// ImageBuild represents build in progress
type ImageBuild interface {
	// From executes FROM command
	From(cmd *FromCommand) error

	// Params executes PARAMS command
	Params(cmd *ParamsCommand) error

	// Run executes RUN command
	Run(ctx context.Context, cmd *RunCommand) error

	// Boot executes BOOT command
	Boot(cmd *BootCommand) error
}

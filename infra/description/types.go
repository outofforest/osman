package description

import (
	"github.com/outofforest/osman/infra/types"
)

// DefaultTag is used if user specified empty tag list
const DefaultTag types.Tag = "latest"

// Command is implemented by commands available in SpecFile
type Command interface {
	// Execute executes build command
	Execute(build ImageBuild) error
}

// ImageBuild represents build in progress
type ImageBuild interface {
	// From executes FROM command
	From(cmd *FromCommand) error

	// Params executes PARAMS command
	Params(cmd *ParamsCommand) error

	// Run executes RUN command
	Run(cmd *RunCommand) error
}

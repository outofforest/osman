package config

import (
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// List stores configuration for list command
type List struct {
	// Formatter is the name of formatter to use to convert list into string
	Formatter string

	// BuildIDs is the list of builds to return
	BuildIDs []types.BuildID

	// BuildKeys is the list of build keys to return
	BuildKeys []types.BuildKey
}

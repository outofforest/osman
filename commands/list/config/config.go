package config

import (
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// List stores configuration for list command
type List struct {
	// BuildIDs is the list of builds to list
	BuildIDs []types.BuildID
}

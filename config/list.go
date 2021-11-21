package config

import (
	"fmt"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// NewList returns new list config
func NewList(args Args) List {
	config := List{
		BuildIDs:  make([]types.BuildID, 0, len(args)),
		BuildKeys: make([]types.BuildKey, 0, len(args)),
	}

	for _, arg := range args {
		buildID, err := types.ParseBuildID(arg)
		if err == nil {
			config.BuildIDs = append(config.BuildIDs, buildID)
			continue
		}

		buildKey, err := types.ParseBuildKey(arg)
		if err != nil {
			panic(fmt.Errorf("argument '%s' is neither valid build ID nor build key", arg))
		}
		config.BuildKeys = append(config.BuildKeys, buildKey)
	}
	return config
}

// List stores configuration for list command
type List struct {
	// BuildIDs is the list of builds to return
	BuildIDs []types.BuildID

	// BuildKeys is the list of build keys to return
	BuildKeys []types.BuildKey
}

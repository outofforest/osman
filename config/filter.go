package config

import (
	"fmt"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// NewFilterFactory returns new filter config factory
func NewFilterFactory() *FilterFactory {
	return &FilterFactory{}
}

// FilterFactory collects data for filter config
type FilterFactory struct {
	// Untagged filters untagged builds only
	Untagged bool
}

// NewFilter returns new filter config
func NewFilter(f *FilterFactory, args Args) Filter {
	config := Filter{
		Untagged:  f.Untagged,
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

// Filter stores configuration of filtering criteria
type Filter struct {
	// Untagged filters untagged builds only
	Untagged bool

	// BuildIDs is the list of builds to return
	BuildIDs []types.BuildID

	// BuildKeys is the list of build keys to return
	BuildKeys []types.BuildKey
}

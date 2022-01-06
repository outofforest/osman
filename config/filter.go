package config

import (
	"fmt"
	"sort"

	"github.com/outofforest/osman/infra/types"
)

const (
	// BuildTypeImage represents image build
	BuildTypeImage = "image"

	// BuildTypeMount represents mount build
	BuildTypeMount = "mount"

	// BuildTypeVM represents vm build
	BuildTypeVM = "vm"
)

var typeMapping = map[string]types.BuildType{
	BuildTypeImage: types.BuildTypeImage,
	BuildTypeMount: types.BuildTypeMount,
	BuildTypeVM:    types.BuildTypeVM,
}

// BuildTypes returns valid build types
func BuildTypes() []string {
	res := make([]string, 0, len(typeMapping))
	for t := range typeMapping {
		res = append(res, t)
	}
	sort.Strings(res)
	return res
}

// FilterFactory collects data for filter config
type FilterFactory struct {
	// Untagged filters untagged builds only
	Untagged bool

	// Types is the list of build types to return
	Types []string
}

// Config returns new filter config
func (f *FilterFactory) Config(args Args) Filter {
	config := Filter{
		Untagged:  f.Untagged,
		Types:     make([]types.BuildType, 0, len(f.Types)),
		BuildIDs:  make([]types.BuildID, 0, len(args)),
		BuildKeys: make([]types.BuildKey, 0, len(args)),
	}
	for _, t := range f.Types {
		buildType, exists := typeMapping[t]
		if !exists {
			panic(fmt.Errorf("build type '%s' is invalid", t))
		}
		config.Types = append(config.Types, buildType)
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

	// Types is the list of build types to return
	Types []types.BuildType

	// BuildIDs is the list of builds to return
	BuildIDs []types.BuildID

	// BuildKeys is the list of build keys to return
	BuildKeys []types.BuildKey
}

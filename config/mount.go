package config

import (
	"fmt"

	"github.com/wojciech-malota-wojcik/imagebuilder/infra/description"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// NewMount creates mount config
func NewMount(args Args) Mount {
	config := Mount{
		Name: args[1],
	}

	buildID, err := types.ParseBuildID(args[0])
	if err == nil {
		config.BuildID = buildID
		return config
	}
	buildKey, err := types.ParseBuildKey(args[0])
	if err != nil {
		panic(fmt.Errorf("argument '%s' is neither valid build ID nor build key", args[0]))
	}
	if buildKey.Tag == "" {
		buildKey.Tag = description.DefaultTag
	}
	config.BuildKey = buildKey
	return config
}

// Mount stores configuration for mount command
type Mount struct {
	// BuildID is the build ID of image to mount
	BuildID types.BuildID

	// BuildKey is the build key of image to mount
	BuildKey types.BuildKey

	// Name is the name of mounted image to create
	Name string
}

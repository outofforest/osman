package config

import (
	"fmt"

	"github.com/outofforest/osman/infra/types"
)

// TagFactory collects data for tag config
type TagFactory struct {
	// Remove is the list of tags to remove
	Remove []string

	// Add is the list of tags to add
	Add []string
}

// Config returns new tag config
func (f *TagFactory) Config() Tag {
	config := Tag{
		Remove: make([]types.Tag, 0, len(f.Remove)),
		Add:    make([]types.Tag, 0, len(f.Add)),
	}
	for _, t := range f.Remove {
		tag := types.Tag((t))
		if !tag.IsValid() {
			panic(fmt.Errorf("invalid tag '%s'", tag))
		}
		config.Remove = append(config.Remove, tag)
	}
	for _, t := range f.Add {
		tag := types.Tag((t))
		if !tag.IsValid() {
			panic(fmt.Errorf("invalid tag '%s'", tag))
		}
		config.Add = append(config.Add, tag)
	}
	return config
}

// Tag stores configuration related to tag operation
type Tag struct {
	// Remove is the list of tags to remove
	Remove []types.Tag

	// Add is the list of tags to add
	Add []types.Tag
}

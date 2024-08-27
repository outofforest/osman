package config

import (
	"github.com/pkg/errors"

	"github.com/outofforest/osman/infra/types"
)

// MountFactory collects data for mount config.
type MountFactory struct {
	// Tags are the tags applied to mounts.
	Tags []string

	// Boot means that the mount is created for booting host machine.
	Boot bool
}

// Config returns new mount config.
func (f *MountFactory) Config(args Args) Mount {
	tags := make(types.Tags, 0, len(f.Tags))
	for _, tag := range f.Tags {
		t := types.Tag(tag)
		if !t.IsValid() {
			panic(errors.Errorf("tag %s is invalid", t))
		}
		tags = append(tags, t)
	}
	config := Mount{
		Tags: tags,
		Type: types.BuildTypeMount,
	}
	if f.Boot {
		config.Type = types.BuildTypeBoot
	}

	return config
}

// Mount stores configuration for mount command.
type Mount struct {
	// Type is the type of mount.
	Type types.BuildType

	// Tags are the tags applied to mounts.
	Tags types.Tags
}

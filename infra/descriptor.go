package infra

import (
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/runtime"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// Describe creates descriptor for image
func Describe(name string, tags []types.Tag, commands ...Command) *Descriptor {
	if len(tags) == 0 {
		tags = []types.Tag{runtime.DefaultTag}
	}
	return &Descriptor{
		name:     name,
		tags:     tags,
		commands: commands,
	}
}

// Descriptor describes future image
type Descriptor struct {
	name     string
	tags     []types.Tag
	commands []Command
}

// Name returns name of the image
func (d *Descriptor) Name() string {
	return d.name
}

// Tags returns tags of the image
func (d *Descriptor) Tags() []types.Tag {
	return d.tags
}

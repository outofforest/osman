package description

import (
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

// Describe creates descriptor for image
func Describe(name string, tags types.Tags, commands ...Command) *Descriptor {
	return &Descriptor{
		name:     name,
		tags:     tags,
		commands: commands,
	}
}

// Descriptor describes future image
type Descriptor struct {
	name     string
	tags     types.Tags
	commands []Command
}

// Name returns name of the image
func (d *Descriptor) Name() string {
	return d.name
}

// Tags returns tags of the image
func (d *Descriptor) Tags() types.Tags {
	return d.tags
}

// Commands returns commands
func (d *Descriptor) Commands() []Command {
	return d.commands
}

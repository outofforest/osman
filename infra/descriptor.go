package infra

// Describe creates descriptor for image
func Describe(name string, commands ...Command) *Descriptor {
	return &Descriptor{
		name:     name,
		commands: commands,
	}
}

// Descriptor describes future image
type Descriptor struct {
	name     string
	commands []Command
}

// Name returns name of the image
func (d *Descriptor) Name() string {
	return d.name
}

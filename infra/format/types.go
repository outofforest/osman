package format

import (
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/list/config"
	"github.com/wojciech-malota-wojcik/ioc"
)

// Formatter formats slice into string
type Formatter interface {
	// Format formats build list into string
	Format(slice interface{}) string
}

// Resolve resolves concrete formatter based on config
func Resolve(c *ioc.Container, config config.List) Formatter {
	var formatter Formatter
	c.ResolveNamed(config.Formatter, &formatter)
	return formatter
}

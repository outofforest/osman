package format

import (
	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/ioc/v2"
)

// Formatter formats slice into string
type Formatter interface {
	// Format formats build list into string
	Format(slice interface{}) string
}

// Resolve resolves concrete formatter based on config
func Resolve(c *ioc.Container, config config.Format) Formatter {
	var formatter Formatter
	c.ResolveNamed(config.Formatter, &formatter)
	return formatter
}

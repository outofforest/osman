package format

import (
	"github.com/outofforest/ioc/v2"

	"github.com/outofforest/osman/config"
)

// Formatter formats slice into string
type Formatter interface {
	// Format formats build list into string
	Format(slice interface{}, fieldsToPrint ...string) string
}

// Resolve resolves concrete formatter based on config
func Resolve(c *ioc.Container, config config.Format) Formatter {
	var formatter Formatter
	c.ResolveNamed(config.Formatter, &formatter)
	return formatter
}

package config

// FormatFactory collects data for format config.
type FormatFactory struct {
	// Formatter is the name of formatter to use to convert list into string.
	Formatter string
}

// Config returns new format config.
func (f *FormatFactory) Config() Format {
	return Format{
		Formatter: f.Formatter,
	}
}

// Format stores configuration specific to formatting.
type Format struct {
	// Formatter is the name of formatter to use to convert list into string.
	Formatter string
}

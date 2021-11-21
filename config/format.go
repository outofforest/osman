package config

// NewFormatFactory returns new format config factory
func NewFormatFactory() *FormatFactory {
	return &FormatFactory{}
}

// FormatFactory collects data for format config
type FormatFactory struct {
	// Formatter is the name of formatter to use to convert list into string
	Formatter string
}

// NewFormat returns new format config
func NewFormat(f *FormatFactory, args Args) Format {
	return Format{
		Formatter: f.Formatter,
	}
}

// Format stores configuration specific to formatting
type Format struct {
	// Formatter is the name of formatter to use to convert list into string
	Formatter string
}

package config

// LoggingFactory collects data for logging config
type LoggingFactory struct {
	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

// Config returns new logging config
func (f *LoggingFactory) Config() Logging {
	return Logging{
		Verbose: f.VerboseLogging,
	}
}

// Logging stores configuration ogf logging
type Logging struct {
	// Verbose turns on verbose logging
	Verbose bool
}

package config

// NewRootFactory returns new root config factory
func NewRootFactory() *RootFactory {
	return &RootFactory{}
}

// RootFactory collects data for root config
type RootFactory struct {
	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

// NewRoot returns new root config
func NewRoot(f *RootFactory) Root {
	return Root{
		VerboseLogging: f.VerboseLogging,
	}
}

// Root stores configuration common to all commands
type Root struct {
	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

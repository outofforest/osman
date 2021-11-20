package config

// Root stores configuration common to all commands
type Root struct {
	// RootDir is the root directory for images
	RootDir string

	// VerboseLogging turns on verbose logging
	VerboseLogging bool
}

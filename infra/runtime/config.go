package runtime

// Config stores configuration
type Config struct {
	// RootDir is the root directory for images
	RootDir string

	// Dockerfile path to build
	Dockerfile string
}

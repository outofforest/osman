package config

// NewDropFactory returns new drop config factory
func NewDropFactory() *DropFactory {
	return &DropFactory{}
}

// DropFactory collects data for drop config
type DropFactory struct {
	// If no filter is provided it is required to set this flag to drop builds
	All bool
}

// NewDrop returns new drop config
func NewDrop(f *DropFactory) Drop {
	return Drop{
		All: f.All,
	}
}

// Drop stores configuration related to drop operation
type Drop struct {
	// If no filter is provided it is required to set this flag to drop builds
	All bool
}

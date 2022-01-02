package config

// StorageFactory collects data for storage config
type StorageFactory struct {
	// Root is the root location for images
	Root string

	// Driver specifies storage driver to use
	Driver string
}

// Config returns new storage config
func (f *StorageFactory) Config() Storage {
	return Storage{
		Root:   f.Root,
		Driver: f.Driver,
	}
}

// Storage stores configuration related to storage drivers
type Storage struct {
	// Root is the root location for images
	Root string

	// Driver specifies storage driver to use
	Driver string
}

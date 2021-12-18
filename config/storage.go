package config

// NewStorageFactory returns new storage config factory
func NewStorageFactory() *StorageFactory {
	return &StorageFactory{}
}

// StorageFactory collects data for storage config
type StorageFactory struct {
	// Root is the root location for images
	Root string

	// Driver specifies storage driver to use
	Driver string
}

// NewStorage returns new storage config
func NewStorage(f *StorageFactory) Storage {
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

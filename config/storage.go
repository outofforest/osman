package config

// NewStorageFactory returns new storage config factory
func NewStorageFactory() *StorageFactory {
	return &StorageFactory{}
}

// StorageFactory collects data for storage config
type StorageFactory struct {
	// RootDir is the root directory for images
	RootDir string
}

// NewStorage returns new storage config
func NewStorage(f *StorageFactory) Storage {
	return Storage{
		RootDir: f.RootDir,
	}
}

// Storage stores configuration related to storage drivers
type Storage struct {
	// RootDir is the root directory for images
	RootDir string
}

package config

// DropFactory collects data for drop config.
type DropFactory struct {
	// If no filter is provided it is required to set this flag to drop builds.
	All bool

	// LibvirtAddr is the address libvirt listens on.
	LibvirtAddr string
}

// Config returns new drop config.
func (f *DropFactory) Config() Drop {
	return Drop{
		All:         f.All,
		LibvirtAddr: f.LibvirtAddr,
	}
}

// Drop stores configuration related to drop operation.
type Drop struct {
	// If no filter is provided it is required to set this flag to drop builds.
	All bool

	// LibvirtAddr is the address libvirt listens on.
	LibvirtAddr string
}

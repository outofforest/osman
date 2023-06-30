package config

// StopFactory collects data for stop config
type StopFactory struct {
	// If no filter is provided it is required to set this flag to stop builds
	All bool

	// LibvirtAddr is the address libvirt listens on
	LibvirtAddr string
}

// Config returns new stop config
func (f *StopFactory) Config() Stop {
	config := Stop{
		All:         f.All,
		LibvirtAddr: f.LibvirtAddr,
	}
	return config
}

// Stop stores configuration for stop command
type Stop struct {
	// If no filter is provided it is required to set this flag to stop builds
	All bool

	// LibvirtAddr is the address libvirt listens on
	LibvirtAddr string
}

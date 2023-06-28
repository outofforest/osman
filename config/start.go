package config

import (
	"github.com/pkg/errors"

	"github.com/outofforest/osman/infra/types"
)

// StartFactory collects data for start config
type StartFactory struct {
	// Tag is the tag applied to started VMs
	Tag string

	// XMLDir is a directory where VM definition is taken from if xml file is not provided explicitly
	XMLDir string

	// VolumeDir is a directory where vm-specific folder exists containing subfolders to be mounted as filesystems in the VM
	VolumeDir string

	// VMFile is the file containing VM definition
	VMFile string

	// LibvirtAddr is the address libvirt listens on
	LibvirtAddr string
}

// Config returns new start config
func (f *StartFactory) Config(args Args) Start {
	config := Start{
		XMLDir:      f.XMLDir,
		VolumeDir:   f.VolumeDir,
		LibvirtAddr: f.LibvirtAddr,
	}
	if f.Tag != "" {
		config.Tag = types.Tag(f.Tag)
		if !config.Tag.IsValid() {
			panic(errors.Errorf("tag %s is invalid", config.Tag))
		}
	}
	if f.VMFile != "auto" {
		config.VMFile = f.VMFile
	}
	return config
}

// Start stores configuration for start command
type Start struct {
	// Tag is the tag applied to started VMs
	Tag types.Tag

	// XMLDir is a directory where VM definition is taken from if xml file is not provided explicitly
	XMLDir string

	// VolumeDir is a directory where vm-specific folder exists containing subfolders to be mounted as filesystems in the VM
	VolumeDir string

	// VMFile is the file containing VM definition
	VMFile string

	// LibvirtAddr is the address libvirt listens on
	LibvirtAddr string
}

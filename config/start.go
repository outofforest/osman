package config

import (
	"github.com/pkg/errors"

	"github.com/outofforest/osman/infra/description"
	"github.com/outofforest/osman/infra/types"
)

// StartFactory collects data for start config
type StartFactory struct {
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
	if f.VMFile != "auto" {
		config.VMFile = f.VMFile
	}
	if len(args) >= 2 {
		var err error
		config.MountKey, err = types.ParseBuildKey(args[1])
		if err != nil {
			panic(err)
		}
	}

	buildID, err := types.ParseBuildID(args[0])
	if err == nil {
		config.ImageBuildID = buildID
		return config
	}
	buildKey, err := types.ParseBuildKey(args[0])
	if err != nil {
		panic(errors.Errorf("argument '%s' is neither valid build ID nor build key", args[0]))
	}
	if buildKey.Tag == "" {
		buildKey.Tag = description.DefaultTag
	}
	config.ImageBuildKey = buildKey
	return config
}

// Start stores configuration for start command
type Start struct {
	// ImageBuildID is the build ID of image to mount
	ImageBuildID types.BuildID

	// ImageBuildKey is the build key of image to mount
	ImageBuildKey types.BuildKey

	// MountKey is the build key of mounted image
	MountKey types.BuildKey

	// XMLDir is a directory where VM definition is taken from if xml file is not provided explicitly
	XMLDir string

	// VolumeDir is a directory where vm-specific folder exists containing subfolders to be mounted as filesystems in the VM
	VolumeDir string

	// VMFile is the file containing VM definition
	VMFile string

	// LibvirtAddr is the address libvirt listens on
	LibvirtAddr string
}

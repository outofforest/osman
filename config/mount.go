package config

import (
	"github.com/pkg/errors"

	"github.com/outofforest/osman/infra/description"
	"github.com/outofforest/osman/infra/types"
)

// MountFactory collects data for mount config
type MountFactory struct {
	// Boot means that the mount is created for booting host machine
	Boot bool

	// XMLDir is a directory where VM definition is taken from if xml file is not provided explicitly
	XMLDir string

	// VMFile is the file containing VM definition
	VMFile string

	// LibvirtAddr is the address libvirt listens on
	LibvirtAddr string
}

// Config returns new mount config
func (f *MountFactory) Config(args Args) Mount {
	config := Mount{
		XMLDir:      f.XMLDir,
		LibvirtAddr: f.LibvirtAddr,
		Type:        types.BuildTypeMount,
	}
	if f.Boot {
		config.Type = types.BuildTypeBoot
	}
	if f.VMFile != "" {
		if f.Boot {
			panic(errors.New("can't mount as boot and vm at the same time"))
		}
		config.Type = types.BuildTypeVM
		if f.VMFile != "auto" {
			config.VMFile = f.VMFile
		}
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

// Mount stores configuration for mount command
type Mount struct {
	// ImageBuildID is the build ID of image to mount
	ImageBuildID types.BuildID

	// ImageBuildKey is the build key of image to mount
	ImageBuildKey types.BuildKey

	// MountKey is the build key of mounted image
	MountKey types.BuildKey

	// Type is the type of mount
	Type types.BuildType

	// XMLDir is a directory where VM definition is taken from if xml file is not provided explicitly
	XMLDir string

	// VMFile is the file containing VM definition
	VMFile string

	// LibvirtAddr is the address libvirt listens on
	LibvirtAddr string
}

package config

import (
	"fmt"

	"github.com/outofforest/osman/infra/description"
	"github.com/outofforest/osman/infra/types"
)

// MountFactory collects data for mount config
type MountFactory struct {
	// Name for mounted image
	Name string

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
		Name:        f.Name,
		XMLDir:      f.XMLDir,
		LibvirtAddr: f.LibvirtAddr,
		Type:        types.BuildTypeMount,
	}
	if f.VMFile != "" {
		config.Type = types.BuildTypeVM
		if f.VMFile != "auto" {
			config.VMFile = f.VMFile
		}
	}

	buildID, err := types.ParseBuildID(args[0])
	if err == nil {
		config.BuildID = buildID
		return config
	}
	buildKey, err := types.ParseBuildKey(args[0])
	if err != nil {
		panic(fmt.Errorf("argument '%s' is neither valid build ID nor build key", args[0]))
	}
	if buildKey.Tag == "" {
		buildKey.Tag = description.DefaultTag
	}
	config.BuildKey = buildKey
	return config
}

// Mount stores configuration for mount command
type Mount struct {
	// Name for mounted image
	Name string

	// BuildID is the build ID of image to mount
	BuildID types.BuildID

	// BuildKey is the build key of image to mount
	BuildKey types.BuildKey

	// Type is the type of mount
	Type types.BuildType

	// XMLDir is a directory where VM definition is taken from if xml file is not provided explicitly
	XMLDir string

	// VMFile is the file containing VM definition
	VMFile string

	// LibvirtAddr is the address libvirt listens on
	LibvirtAddr string
}

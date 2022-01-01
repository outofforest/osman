package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/outofforest/go-zfs/v2"
	"github.com/ridge/must"
	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

const propertyName = "co.exw:info"

// NewZFSDriver returns new storage driver based on zfs datasets
func NewZFSDriver(config config.Storage) Driver {
	return &zfsDriver{
		config: config,
	}
}

type zfsDriver struct {
	config config.Storage
}

// Builds returns available builds
func (d *zfsDriver) Builds() ([]types.BuildID, error) {
	filesystems, err := zfs.Filesystems("")
	if err != nil {
		return nil, err
	}

	builds := []types.BuildID{}
	prefix := d.config.Root + "/"
	prefixLen := len(prefix)
	for _, ds := range filesystems {
		if strings.HasPrefix(ds.Name, prefix) {
			buildID, err := types.ParseBuildID(ds.Name[prefixLen:])
			if err != nil {
				return nil, err
			}
			builds = append(builds, buildID)
		}
	}
	return builds, nil
}

// Info returns information about build
func (d *zfsDriver) Info(buildID types.BuildID) (types.BuildInfo, error) {
	filesystems, err := zfs.Filesystems(d.config.Root + "/" + string(buildID))
	if err != nil {
		return types.BuildInfo{}, err
	}

	filesystem := filesystems[0]
	info, err := filesystem.GetProperty(propertyName)
	if err != nil {
		return types.BuildInfo{}, err
	}

	var buildInfo types.BuildInfo
	if err := json.Unmarshal([]byte(info), &buildInfo); err != nil {
		return types.BuildInfo{}, err
	}

	mounted := ""
	if buildID.Type().Properties().Mountable {
		mounted = filesystem.Mountpoint
	}
	buildInfo.Mounted = mounted

	return buildInfo, nil
}

// BuildID returns build ID for build given by name and tag
func (d *zfsDriver) BuildID(buildKey types.BuildKey) (types.BuildID, error) {
	builds, err := d.Builds()
	if err != nil {
		return "", err
	}

	for _, buildID := range builds {
		info, err := d.Info(buildID)
		if err != nil {
			return "", err
		}
		if info.Name == buildKey.Name && inTags(info.Tags, buildKey.Tag) {
			return buildID, nil
		}
	}
	return "", fmt.Errorf("image %s does not exist: %w", buildKey, types.ErrImageDoesNotExist)
}

// CreateEmpty creates blank build
func (d *zfsDriver) CreateEmpty(imageName string, buildID types.BuildID) (FinalizeFn, string, error) {
	filesystem, err := zfs.CreateFilesystem(d.config.Root+"/"+string(buildID), map[string]string{
		propertyName: string(must.Bytes(json.Marshal(types.BuildInfo{
			BuildID:   buildID,
			Name:      imageName,
			CreatedAt: time.Now(),
		}))),
	})
	if err != nil {
		return nil, "", err
	}

	return func() error {
		if _, err := filesystem.Unmount(false); err != nil {
			return err
		}
		if err := filesystem.SetProperty("canmount", "off"); err != nil {
			return err
		}
		_, err := filesystem.Snapshot("image", false)
		return err
	}, filesystem.Mountpoint, nil
}

// Clone clones source build to destination build
func (d *zfsDriver) Clone(srcBuildID types.BuildID, dstImageName string, dstBuildID types.BuildID) (FinalizeFn, string, error) {
	snapshots, err := zfs.Snapshots(d.config.Root + "/" + string(srcBuildID) + "@image")
	if err != nil {
		return nil, "", err
	}

	snapshot := snapshots[0]
	filesystem, err := snapshot.Clone(d.config.Root+"/"+string(dstBuildID), map[string]string{
		propertyName: string(must.Bytes(json.Marshal(types.BuildInfo{
			BuildID:   dstBuildID,
			BasedOn:   srcBuildID,
			Name:      dstImageName,
			CreatedAt: time.Now(),
		}))),
	})
	if err != nil {
		return nil, "", err
	}

	return func() error {
		properties := dstBuildID.Type().Properties()
		if !properties.Mountable {
			if _, err := filesystem.Unmount(false); err != nil {
				return err
			}
			if err := filesystem.SetProperty("canmount", "off"); err != nil {
				return err
			}
		}
		if properties.Cloneable {
			if _, err := filesystem.Snapshot("image", false); err != nil {
				return err
			}
		}
		return nil
	}, filesystem.Mountpoint, nil
}

// StoreManifest stores manifest of build
func (d *zfsDriver) StoreManifest(manifest types.ImageManifest) error {
	info, err := d.Info(manifest.BuildID)
	if err != nil {
		return err
	}
	info.Params = manifest.Params
	return d.setInfo(info)
}

// Tag tags build with tag
func (d *zfsDriver) Tag(buildID types.BuildID, tag types.Tag) error {
	info, err := d.Info(buildID)
	if err != nil {
		return err
	}

	existingBuildID, err := d.BuildID(types.NewBuildKey(info.Name, tag))
	switch {
	case err == nil:
		existingInfo, err := d.Info(existingBuildID)
		if err != nil {
			return err
		}
		tags := make(types.Tags, 0, len(existingInfo.Tags)-1)
		for _, t := range existingInfo.Tags {
			if t != tag {
				tags = append(tags, t)
			}
		}
		existingInfo.Tags = tags
		if err := d.setInfo(existingInfo); err != nil {
			return err
		}
	case errors.Is(err, types.ErrImageDoesNotExist):
	default:
		return err
	}

	info.Tags = append(info.Tags, tag)
	return d.setInfo(info)
}

// Drop drops image
func (d *zfsDriver) Drop(buildID types.BuildID) error {
	filesystems, err := zfs.Filesystems(d.config.Root + "/" + string(buildID))
	if err != nil {
		return fmt.Errorf("build %s does not exist: %w", buildID, types.ErrImageDoesNotExist)
	}

	filesystem := filesystems[0]
	if err := filesystem.Destroy(zfs.DestroyRecursive); err != nil {
		return fmt.Errorf("build %s have children: %w", buildID, ErrImageHasChildren)
	}
	return nil
}

func (d *zfsDriver) setInfo(info types.BuildInfo) error {
	filesystems, err := zfs.Filesystems(d.config.Root + "/" + string(info.BuildID))
	if err != nil {
		return err
	}

	return filesystems[0].SetProperty(propertyName, string(must.Bytes(json.Marshal(info))))
}

func inTags(slice types.Tags, el types.Tag) bool {
	for _, s := range slice {
		if s == el {
			return true
		}
	}
	return false
}

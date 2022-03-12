package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/outofforest/go-zfs/v3"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/types"
	"github.com/ridge/must"
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
func (d *zfsDriver) Builds(ctx context.Context) ([]types.BuildID, error) {
	rootFs, err := zfs.GetFilesystem(ctx, d.config.Root)
	if err != nil {
		return nil, err
	}

	builds := []types.BuildID{}
	prefixLen := len(d.config.Root + "/")
	filesystems, err := rootFs.Children(ctx)
	if err != nil {
		return nil, err
	}
	for _, fs := range filesystems {
		buildID, err := types.ParseBuildID(fs.Info.Name[prefixLen:])
		if err != nil {
			return nil, err
		}
		builds = append(builds, buildID)
	}
	return builds, nil
}

// Info returns information about build
func (d *zfsDriver) Info(ctx context.Context, buildID types.BuildID) (types.BuildInfo, error) {
	filesystem, err := zfs.GetFilesystem(ctx, d.config.Root+"/"+string(buildID))
	if err != nil {
		return types.BuildInfo{}, err
	}

	info, err := filesystem.GetProperty(ctx, propertyName)
	if err != nil {
		return types.BuildInfo{}, err
	}

	var buildInfo types.BuildInfo
	if err := json.Unmarshal([]byte(info), &buildInfo); err != nil {
		return types.BuildInfo{}, err
	}

	mounted := ""
	if buildID.Type().Properties().Mountable {
		mounted = filesystem.Info.Mountpoint
	}
	buildInfo.Mounted = mounted

	return buildInfo, nil
}

// BuildID returns build ID for build given by name and tag
func (d *zfsDriver) BuildID(ctx context.Context, buildKey types.BuildKey) (types.BuildID, error) {
	builds, err := d.Builds(ctx)
	if err != nil {
		return "", err
	}

	for _, buildID := range builds {
		info, err := d.Info(ctx, buildID)
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
func (d *zfsDriver) CreateEmpty(ctx context.Context, imageName string, buildID types.BuildID) (FinalizeFn, string, error) {
	filesystem, err := zfs.CreateFilesystem(ctx, d.config.Root+"/"+string(buildID), map[string]string{
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
		if err := filesystem.Unmount(ctx); err != nil {
			return err
		}
		if err := filesystem.SetProperty(ctx, "canmount", "off"); err != nil {
			return err
		}
		_, err := filesystem.Snapshot(ctx, "image")
		return err
	}, filesystem.Info.Mountpoint, nil
}

// Clone clones source build to destination build
func (d *zfsDriver) Clone(ctx context.Context, srcBuildID types.BuildID, dstImageName string, dstBuildID types.BuildID) (FinalizeFn, string, error) {
	snapshot, err := zfs.GetSnapshot(ctx, d.config.Root+"/"+string(srcBuildID)+"@image")
	if err != nil {
		return nil, "", err
	}

	filesystem, err := snapshot.Clone(ctx, d.config.Root+"/"+string(dstBuildID))
	if err != nil {
		return nil, "", err
	}
	err = filesystem.SetProperty(ctx, propertyName, string(must.Bytes(json.Marshal(types.BuildInfo{
		BuildID:   dstBuildID,
		BasedOn:   srcBuildID,
		Name:      dstImageName,
		CreatedAt: time.Now(),
	}))))
	if err != nil {
		return nil, "", err
	}

	return func() error {
		properties := dstBuildID.Type().Properties()
		if !properties.Mountable {
			if err := filesystem.Unmount(ctx); err != nil {
				return err
			}
			if err := filesystem.SetProperty(ctx, "canmount", "off"); err != nil {
				return err
			}
		}
		if properties.Cloneable {
			if _, err := filesystem.Snapshot(ctx, "image"); err != nil {
				return err
			}
		}
		return nil
	}, filesystem.Info.Mountpoint, nil
}

// StoreManifest stores manifest of build
func (d *zfsDriver) StoreManifest(ctx context.Context, manifest types.ImageManifest) error {
	info, err := d.Info(ctx, manifest.BuildID)
	if err != nil {
		return err
	}
	info.Params = manifest.Params
	return d.setInfo(ctx, info)
}

// Tag tags build with tag
func (d *zfsDriver) Tag(ctx context.Context, buildID types.BuildID, tag types.Tag) error {
	info, err := d.Info(ctx, buildID)
	if err != nil {
		return err
	}

	existingBuildID, err := d.BuildID(ctx, types.NewBuildKey(info.Name, tag))
	switch {
	case err == nil:
		existingInfo, err := d.Info(ctx, existingBuildID)
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
		if err := d.setInfo(ctx, existingInfo); err != nil {
			return err
		}
	case errors.Is(err, types.ErrImageDoesNotExist):
	default:
		return err
	}

	info.Tags = append(info.Tags, tag)
	return d.setInfo(ctx, info)
}

// Drop drops image
func (d *zfsDriver) Drop(ctx context.Context, buildID types.BuildID) error {
	filesystem, err := zfs.GetFilesystem(ctx, d.config.Root+"/"+string(buildID))
	if err != nil {
		return fmt.Errorf("build %s does not exist: %w", buildID, types.ErrImageDoesNotExist)
	}

	if err := filesystem.Destroy(ctx, zfs.DestroyRecursive); err != nil {
		return fmt.Errorf("build %s have children: %w", buildID, ErrImageHasChildren)
	}
	return nil
}

func (d *zfsDriver) setInfo(ctx context.Context, info types.BuildInfo) error {
	filesystem, err := zfs.GetFilesystem(ctx, d.config.Root+"/"+string(info.BuildID))
	if err != nil {
		return err
	}

	return filesystem.SetProperty(ctx, propertyName, string(must.Bytes(json.Marshal(info))))
}

func inTags(slice types.Tags, el types.Tag) bool {
	for _, s := range slice {
		if s == el {
			return true
		}
	}
	return false
}

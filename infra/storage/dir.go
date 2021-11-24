package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/otiai10/copy"
	"github.com/wojciech-malota-wojcik/imagebuilder/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

const (
	subDirCatalog   = "catalog"
	subDirLinks     = "links"
	subDirBuilds    = "builds"
	subDirManifests = "manifests"
	subDirBuild     = "build"
	subDirChildren  = "children"
)

// FIXME (wojciech): RootDir may be a symlink. Resolve it before using

// NewDirDriver returns new storage driver based on directories
func NewDirDriver(config config.Storage) Driver {
	return &dirDriver{
		config: config,
	}
}

type dirDriver struct {
	config config.Storage
}

// Builds returns available builds
func (d *dirDriver) Builds() ([]types.BuildID, error) {
	buildLinks := filepath.Join(d.config.RootDir, subDirLinks)
	dir, err := os.Open(buildLinks)
	if err != nil {
		if os.IsNotExist(err) {
			return []types.BuildID{}, nil
		}
		return nil, err
	}
	defer dir.Close()

	res := []types.BuildID{}
	var entries []os.DirEntry
	for entries, err = dir.ReadDir(20); err == nil; entries, err = dir.ReadDir(20) {
		for _, e := range entries {
			res = append(res, types.BuildID(e.Name()))
		}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return res, nil
}

// Info returns information about build
func (d *dirDriver) Info(buildID types.BuildID) (types.BuildInfo, error) {
	catalogLink := filepath.Join(d.config.RootDir, subDirLinks, string(buildID))
	stat, err := os.Stat(catalogLink)
	if err != nil {
		return types.BuildInfo{}, err
	}
	imageLinkStatT, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return types.BuildInfo{}, fmt.Errorf("stat can't be retrieved: %s", catalogLink)
	}

	catalogDir, err := filepath.EvalSymlinks(catalogLink)
	if err != nil {
		return types.BuildInfo{}, err
	}

	manifest, err := d.Manifest(buildID)
	if err != nil {
		return types.BuildInfo{}, err
	}

	res := types.BuildInfo{
		BuildID:   buildID,
		BasedOn:   manifest.BasedOn,
		CreatedAt: time.Unix(imageLinkStatT.Ctim.Sec, imageLinkStatT.Ctim.Nsec),
		Name:      filepath.Base(catalogDir),
		Tags:      types.Tags{},
		Params:    manifest.Params,
	}

	dir, err := os.Open(catalogDir)
	if err != nil {
		return types.BuildInfo{}, err
	}
	defer dir.Close()

	var entries []os.DirEntry
	for entries, err = dir.ReadDir(20); err == nil; entries, err = dir.ReadDir(20) {
		for _, e := range entries {
			tagBuildID, err := os.Readlink(filepath.Join(catalogDir, e.Name()))
			if err != nil {
				return types.BuildInfo{}, err
			}
			if tagBuildID != string(buildID) {
				continue
			}
			res.Tags = append(res.Tags, types.Tag(e.Name()))
		}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return types.BuildInfo{}, err
	}
	return res, nil
}

// BuildID returns build ID for build given by name and tag
func (d *dirDriver) BuildID(buildKey types.BuildKey) (types.BuildID, error) {
	buildDir, err := filepath.EvalSymlinks(filepath.Join(d.config.RootDir, subDirCatalog, buildKey.Name, string(buildKey.Tag)))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("image %s does not exist: %w", buildKey, ErrSourceImageDoesNotExist)
		}
		return "", err
	}
	return types.BuildID(filepath.Base(buildDir)), nil
}

// Mount mounts build in filesystem
func (d *dirDriver) Mount(buildID types.BuildID, dstPath string) (UnmountFn, error) {
	if err := syscall.Mount(filepath.Join(d.config.RootDir, subDirLinks, string(buildID), string(buildID), subDirBuild), dstPath, "", syscall.MS_BIND, ""); err != nil {
		return nil, err
	}
	return func() error {
		return syscall.Unmount(dstPath, 0)
	}, nil
}

// CreateEmpty creates blank build
func (d *dirDriver) CreateEmpty(imageName string, buildID types.BuildID) error {
	buildDir := filepath.Join(subDirBuilds, string(buildID))
	catalogLink := filepath.Join(subDirLinks, string(buildID))
	catalogDir := filepath.Join(subDirCatalog, imageName)
	buildLink := filepath.Join(catalogDir, string(buildID))

	if err := d.symlink(filepath.Join("..", catalogDir), filepath.Join(d.config.RootDir, catalogLink)); err != nil {
		return err
	}
	if err := d.symlink(filepath.Join("..", "..", buildDir), filepath.Join(d.config.RootDir, buildLink)); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Join(d.config.RootDir, buildDir, subDirBuild), 0o700)
}

// Clone clones source build to destination build
func (d *dirDriver) Clone(srcBuildID types.BuildID, dstImageName string, dstBuildID types.BuildID) error {
	srcBuildDir, err := filepath.EvalSymlinks(filepath.Join(d.config.RootDir, subDirLinks, string(srcBuildID), string(srcBuildID)))
	if err != nil {
		return err
	}
	srcBuildDir, err = filepath.Rel(d.config.RootDir, srcBuildDir)
	if err != nil {
		return err
	}
	buildDir := filepath.Join(srcBuildDir, subDirChildren, string(dstBuildID))
	catalogLink := filepath.Join(subDirLinks, string(dstBuildID))
	catalogDir := filepath.Join(subDirCatalog, dstImageName)
	buildLink := filepath.Join(catalogDir, string(dstBuildID))

	if err := d.symlink(filepath.Join("..", catalogDir), filepath.Join(d.config.RootDir, catalogLink)); err != nil {
		return err
	}
	if err := d.symlink(filepath.Join("..", "..", buildDir), filepath.Join(d.config.RootDir, buildLink)); err != nil {
		return err
	}
	dst := filepath.Join(d.config.RootDir, buildDir, subDirBuild)
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	return copy.Copy(filepath.Join(d.config.RootDir, srcBuildDir, subDirBuild), dst, copy.Options{PreserveTimes: true, PreserveOwner: true})
}

// Manifest returns manifest of build
func (d *dirDriver) Manifest(buildID types.BuildID) (types.ImageManifest, error) {
	manifestRaw, err := ioutil.ReadFile(filepath.Join(d.config.RootDir, subDirManifests, string(buildID)))
	if err != nil {
		return types.ImageManifest{}, err
	}
	var manifest types.ImageManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return types.ImageManifest{}, err
	}
	return manifest, err
}

// StoreManifest stores manifest of build
func (d *dirDriver) StoreManifest(manifest types.ImageManifest) error {
	manifestFile := filepath.Join(d.config.RootDir, subDirManifests, string(manifest.BuildID))
	if err := os.MkdirAll(filepath.Dir(manifestFile), 0o700); err != nil && !os.IsExist(err) {
		return err
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(manifestFile, manifestRaw, 0o600)
}

// Tag tags build with tag
func (d *dirDriver) Tag(buildID types.BuildID, tag types.Tag) error {
	tagLink := filepath.Join(d.config.RootDir, subDirLinks, string(buildID), string(tag))
	if err := os.Remove(tagLink); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(string(buildID), tagLink)
}

// Drop drops image
func (d *dirDriver) Drop(buildID types.BuildID) (retErr error) {
	buildsDir := filepath.Join(d.config.RootDir, subDirBuilds)
	catalogLink := filepath.Join(d.config.RootDir, subDirLinks, string(buildID))
	buildDir, err := filepath.EvalSymlinks(filepath.Join(catalogLink, string(buildID)))
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return err
	default:
		_, err := os.Stat(filepath.Join(buildDir, subDirChildren))
		switch {
		case err == nil:
			return fmt.Errorf("build %s has children: %w", buildID, ErrImageHasChildren)
		case os.IsNotExist(err):
		default:
			return err
		}
		if err := os.RemoveAll(buildDir); err != nil && !os.IsNotExist(err) {
			return err
		}
		dir := buildDir
	loop:
		for {
			dir = filepath.Dir(dir)
			if dir == buildsDir {
				break
			}
			err := os.Remove(dir)
			switch {
			case err == nil:
			case os.IsNotExist(err):
				break loop
			default:
				dirH, err2 := os.Open(dir)
				if err2 != nil {
					if os.IsNotExist(err2) {
						break loop
					}
				}
				_, err2 = dirH.Readdir(1)
				switch {
				case errors.Is(err2, io.EOF):
					return err
				case err2 == nil:
					break loop
				default:
					return err2
				}
			}
		}
	}

	if err := os.Remove(filepath.Join(d.config.RootDir, subDirManifests, string(buildID))); err != nil && !os.IsNotExist(err) {
		return err
	}

	catalogDir, err := filepath.EvalSymlinks(catalogLink)
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return err
	default:
		if err := os.Remove(filepath.Join(catalogDir, string(buildID))); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	dir, err := os.Open(catalogDir)
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return err
	default:
		var entries []os.DirEntry
		empty := true
		for entries, err = dir.ReadDir(20); err == nil; entries, err = dir.ReadDir(20) {
			for _, e := range entries {
				tagLink := filepath.Join(catalogDir, e.Name())
				tagBuildID, err := os.Readlink(tagLink)
				if err != nil {
					return err
				}
				if tagBuildID != string(buildID) {
					empty = false
					continue
				}
				if err := os.Remove(tagLink); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if empty {
			if err := os.Remove(catalogDir); err != nil {
				return err
			}
		}
	}
	return os.Remove(catalogLink)
}

func (d *dirDriver) symlink(oldname, newname string) error {
	if err := os.MkdirAll(filepath.Dir(newname), 0o700); err != nil && !os.IsExist(err) {
		return err
	}
	return os.Symlink(oldname, newname)
}

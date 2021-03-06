package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/outofforest/isolator"
	"github.com/outofforest/isolator/client/wire"
	"github.com/pkg/errors"

	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/types"
)

const (
	subDirCatalog   = "catalog"
	subDirLinks     = "links"
	subDirBuilds    = "builds"
	subDirManifests = "manifests"
	subDirBuild     = "build"
	subDirChildren  = "children"
)

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
func (d *dirDriver) Builds(ctx context.Context) ([]types.BuildID, error) {
	buildLinks := filepath.Join(d.config.Root, subDirLinks)
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
func (d *dirDriver) Info(ctx context.Context, buildID types.BuildID) (types.BuildInfo, error) {
	catalogLink := filepath.Join(d.config.Root, subDirLinks, string(buildID))
	stat, err := os.Stat(catalogLink)
	if err != nil {
		return types.BuildInfo{}, err
	}
	imageLinkStatT, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return types.BuildInfo{}, errors.Errorf("stat can't be retrieved: %s", catalogLink)
	}

	catalogDir, err := filepath.EvalSymlinks(catalogLink)
	if err != nil {
		return types.BuildInfo{}, err
	}

	manifestRaw, err := ioutil.ReadFile(filepath.Join(d.config.Root, subDirManifests, string(buildID)))
	if err != nil {
		return types.BuildInfo{}, err
	}
	var manifest types.ImageManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return types.BuildInfo{}, err
	}

	mounted := ""
	if buildID.Type().Properties().Mountable {
		mounted = filepath.Join(catalogDir, string(buildID))
	}

	res := types.BuildInfo{
		BuildID:   buildID,
		BasedOn:   manifest.BasedOn,
		CreatedAt: time.Unix(imageLinkStatT.Ctim.Sec, imageLinkStatT.Ctim.Nsec),
		Name:      filepath.Base(catalogDir),
		Tags:      types.Tags{},
		Params:    manifest.Params,
		Mounted:   mounted,
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
func (d *dirDriver) BuildID(ctx context.Context, buildKey types.BuildKey) (types.BuildID, error) {
	buildDir, err := filepath.EvalSymlinks(filepath.Join(d.config.Root, subDirCatalog, buildKey.Name, string(buildKey.Tag)))
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.WithStack(fmt.Errorf("image %s does not exist: %w", buildKey, types.ErrImageDoesNotExist))
		}
		return "", err
	}
	return types.BuildID(filepath.Base(buildDir)), nil
}

// CreateEmpty creates blank build
func (d *dirDriver) CreateEmpty(ctx context.Context, imageName string, buildID types.BuildID) (FinalizeFn, string, error) {
	buildDir := filepath.Join(subDirBuilds, string(buildID))
	catalogLink := filepath.Join(subDirLinks, string(buildID))
	catalogDir := filepath.Join(subDirCatalog, imageName)
	buildLink := filepath.Join(catalogDir, string(buildID))

	if err := d.symlink(filepath.Join("..", catalogDir), filepath.Join(d.config.Root, catalogLink)); err != nil {
		return nil, "", err
	}
	if err := d.symlink(filepath.Join("..", "..", buildDir), filepath.Join(d.config.Root, buildLink)); err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(filepath.Join(d.config.Root, buildDir, subDirBuild), 0o700); err != nil {
		return nil, "", err
	}

	path, err := filepath.Abs(filepath.Join(d.config.Root, subDirLinks, string(buildID), string(buildID), subDirBuild))
	if err != nil {
		return nil, "", err
	}

	if err := d.StoreManifest(ctx, types.ImageManifest{
		BuildID: buildID,
	}); err != nil {
		return nil, "", err
	}

	return func() error {
		return nil
	}, path, nil
}

// Clone clones source build to destination build
func (d *dirDriver) Clone(ctx context.Context, srcBuildID types.BuildID, dstImageName string, dstBuildID types.BuildID) (finalizeFn FinalizeFn, mountPath string, retErr error) {
	srcBuildDir, err := filepath.EvalSymlinks(filepath.Join(d.config.Root, subDirLinks, string(srcBuildID), string(srcBuildID)))
	if err != nil {
		return nil, "", err
	}
	srcBuildDir, err = filepath.Rel(d.config.Root, srcBuildDir)
	if err != nil {
		return nil, "", err
	}
	buildDir := filepath.Join(srcBuildDir, subDirChildren, string(dstBuildID))
	catalogLink := filepath.Join(subDirLinks, string(dstBuildID))
	catalogDir := filepath.Join(subDirCatalog, dstImageName)
	buildLink := filepath.Join(catalogDir, string(dstBuildID))

	if err := d.symlink(filepath.Join("..", catalogDir), filepath.Join(d.config.Root, catalogLink)); err != nil {
		return nil, "", err
	}
	if err := d.symlink(filepath.Join("..", "..", buildDir), filepath.Join(d.config.Root, buildLink)); err != nil {
		return nil, "", err
	}
	dst := filepath.Join(buildDir, subDirBuild, "root")
	if err := os.MkdirAll(filepath.Join(d.config.Root, dst), 0o700); err != nil {
		return nil, "", err
	}

	isolator, clean, err := isolator.Start(isolator.Config{Dir: d.config.Root, Executor: wire.Config{Chroot: true}})
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if err := clean(); retErr == nil {
			retErr = err
		}
	}()

	if err := isolator.Send(wire.Copy{
		Src: filepath.Join(srcBuildDir, subDirBuild, "root"),
		Dst: dst,
	}); err != nil {
		return nil, "", err
	}
	msg, err := isolator.Receive()
	if err != nil {
		return nil, "", err
	}
	result, ok := msg.(wire.Result)
	if !ok {
		return nil, "", errors.Errorf("expected Result, got: %T", msg)
	}
	if result.Error != "" {
		return nil, "", errors.New(result.Error)
	}

	path, err := filepath.Abs(filepath.Join(d.config.Root, subDirLinks, string(dstBuildID), string(dstBuildID), subDirBuild))
	if err != nil {
		return nil, "", err
	}

	if err := d.StoreManifest(ctx, types.ImageManifest{
		BuildID: dstBuildID,
		BasedOn: srcBuildID,
	}); err != nil {
		return nil, "", err
	}

	return func() error {
		return nil
	}, path, nil
}

// StoreManifest stores manifest of build
func (d *dirDriver) StoreManifest(ctx context.Context, manifest types.ImageManifest) error {
	manifestFile := filepath.Join(d.config.Root, subDirManifests, string(manifest.BuildID))
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
func (d *dirDriver) Tag(ctx context.Context, buildID types.BuildID, tag types.Tag) error {
	tagLink := filepath.Join(d.config.Root, subDirLinks, string(buildID), string(tag))

	if err := os.Remove(tagLink); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Symlink(string(buildID), tagLink)
}

// Untag removes tag from the build
func (d *dirDriver) Untag(ctx context.Context, buildID types.BuildID, tag types.Tag) error {
	tagLink := filepath.Join(d.config.Root, subDirLinks, string(buildID), string(tag))
	buildDir, err := filepath.EvalSymlinks(tagLink)
	if err != nil {
		return errors.WithStack(fmt.Errorf("build %s is not tagged with %s: %w", buildID, tag, err))
	}
	if string(buildID) != filepath.Base(buildDir) {
		return errors.Errorf("build %s is not tagged with %s", buildID, tag)
	}
	return os.Remove(tagLink)
}

// Drop drops image
func (d *dirDriver) Drop(ctx context.Context, buildID types.BuildID) (retErr error) {
	rootDir, err := d.rootDir()
	if err != nil {
		return err
	}

	buildsDir := filepath.Join(rootDir, subDirBuilds)
	catalogLink := filepath.Join(rootDir, subDirLinks, string(buildID))
	buildDir, err := filepath.EvalSymlinks(filepath.Join(catalogLink, string(buildID)))
	switch {
	case os.IsNotExist(err):
		return errors.WithStack(fmt.Errorf("image %s does not exist: %w", buildID, types.ErrImageDoesNotExist))
	case err != nil:
		return err
	default:
		_, err := os.Stat(filepath.Join(buildDir, subDirChildren))
		switch {
		case err == nil:
			return errors.WithStack(fmt.Errorf("build %s has children: %w", buildID, ErrImageHasChildren))
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

	if err := os.Remove(filepath.Join(rootDir, subDirManifests, string(buildID))); err != nil && !os.IsNotExist(err) {
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

func (d *dirDriver) rootDir() (string, error) {
	rootDir, err := filepath.EvalSymlinks(d.config.Root)
	if os.IsNotExist(err) {
		return d.config.Root, nil
	}
	return rootDir, err
}

func (d *dirDriver) symlink(oldname, newname string) error {
	if err := os.MkdirAll(filepath.Dir(newname), 0o700); err != nil && !os.IsExist(err) {
		return err
	}
	return os.Symlink(oldname, newname)
}

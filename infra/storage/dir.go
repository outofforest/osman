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
	"github.com/wojciech-malota-wojcik/imagebuilder/commands/root/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

const (
	subDirImages    = "images"
	subDirBuilds    = "builds"
	subDirManifests = "manifests"
)

// NewDirDriver returns new storage driver based on directories
func NewDirDriver(config config.Root) Driver {
	return &dirDriver{
		rootPath: config.RootDir,
	}
}

type dirDriver struct {
	rootPath string
}

// Builds returns available builds
func (d *dirDriver) Builds() ([]types.BuildID, error) {
	rootPath, err := filepath.Abs(d.rootPath)
	if err != nil {
		return nil, err
	}
	buildsAbsDir := filepath.Join(rootPath, subDirBuilds)
	dir, err := os.Open(buildsAbsDir)
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
			info, err := e.Info()
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}

			if info.Mode()&os.ModeSymlink != 0 {
				buildAbsLink := filepath.Join(buildsAbsDir, info.Name())
				if _, err := filepath.EvalSymlinks(buildAbsLink); err != nil {
					if os.IsNotExist(err) {
						// dead link, remove it
						if err := os.Remove(buildAbsLink); err != nil && !os.IsNotExist(err) {
							return nil, err
						}
						continue
					}
					return nil, err
				}
				res = append(res, types.BuildID(info.Name()))
			}
		}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return res, nil
}

// Info returns information about build
func (d *dirDriver) Info(buildID types.BuildID) (types.BuildInfo, error) {
	buildAbsLink, err := d.toAbsoluteBuildLink(buildID)
	if err != nil {
		return types.BuildInfo{}, err
	}
	buildAbsDir, err := filepath.EvalSymlinks(buildAbsLink)
	if err != nil {
		return types.BuildInfo{}, err
	}
	tagsAbsDir := filepath.Dir(buildAbsDir)

	manifest, err := d.Manifest(buildID)
	if err != nil {
		return types.BuildInfo{}, err
	}

	stat, err := os.Stat(buildAbsDir)
	if err != nil {
		return types.BuildInfo{}, err
	}
	statT, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		panic("stat can't be retrieved")
	}

	res := types.BuildInfo{
		BuildID:   buildID,
		BasedOn:   manifest.BasedOn,
		CreatedAt: time.Unix(statT.Ctim.Sec, statT.Ctim.Nsec),
		Name:      filepath.Base(tagsAbsDir),
		Tags:      types.Tags{},
	}

	dir, err := os.Open(tagsAbsDir)
	if err != nil {
		return types.BuildInfo{}, err
	}
	defer dir.Close()

	var entries []os.DirEntry
	for entries, err = dir.ReadDir(20); err == nil; entries, err = dir.ReadDir(20) {
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return types.BuildInfo{}, err
			}

			if info.Mode()&os.ModeSymlink != 0 {
				tagAbsLink := filepath.Join(tagsAbsDir, info.Name())
				buildDirFromLink, err := filepath.EvalSymlinks(tagAbsLink)
				if err != nil {
					if os.IsNotExist(err) {
						// dead link, remove it
						if err := os.Remove(tagAbsLink); err != nil && !os.IsNotExist(err) {
							return types.BuildInfo{}, err
						}
						continue
					}
					return types.BuildInfo{}, err
				}
				buildAbsDirFromLink, err := filepath.Abs(buildDirFromLink)
				if err != nil {
					if os.IsNotExist(err) {
						// dead link, remove it
						if err := os.Remove(tagAbsLink); err != nil && !os.IsNotExist(err) {
							return types.BuildInfo{}, err
						}
						continue
					}
					return types.BuildInfo{}, err
				}
				if buildAbsDir == buildAbsDirFromLink {
					res.Tags = append(res.Tags, types.Tag(info.Name()))
				}
			}
		}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return types.BuildInfo{}, err
	}
	return res, nil
}

// BuildID returns build ID for build given by name and tag
func (d *dirDriver) BuildID(buildKey types.BuildKey) (types.BuildID, error) {
	tagLink, err := d.toAbsoluteTagLink(buildKey.Name, buildKey.Tag)
	if err != nil {
		return "", err
	}
	buildDir, err := filepath.EvalSymlinks(tagLink)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("image %s does not exist: %w", buildKey, ErrSourceImageDoesNotExist)
		}
		return "", err
	}
	buildAbsDir, err := filepath.Abs(buildDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("image %s does not exist: %w", buildKey, ErrSourceImageDoesNotExist)
		}
		return "", err
	}
	return types.BuildID(filepath.Base(buildAbsDir)), nil
}

// Mount mounts build in filesystem
func (d *dirDriver) Mount(buildID types.BuildID, dstPath string) (UnmountFn, error) {
	buildLink, err := d.toAbsoluteBuildLink(buildID)
	if err != nil {
		return nil, err
	}
	buildDir, err := filepath.EvalSymlinks(buildLink)
	if err != nil {
		return nil, err
	}
	buildAbsDir, err := filepath.Abs(buildDir)
	if err != nil {
		return nil, err
	}

	if err := syscall.Mount(buildAbsDir, dstPath, "", syscall.MS_BIND, ""); err != nil {
		return nil, err
	}

	return func() error {
		return syscall.Unmount(dstPath, 0)
	}, nil
}

// CreateEmpty creates blank build
func (d *dirDriver) CreateEmpty(imageName string, buildID types.BuildID) error {
	buildAbsLink, err := d.toAbsoluteBuildLink(buildID)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(buildAbsLink), 0o700); err != nil && !os.IsExist(err) {
		return err
	}

	link := filepath.Join("..", d.toRelativeBuildDir(imageName, buildID))

	// Create symlink before creating directory because Drop is based on this symlink
	if err := os.Symlink(link, buildAbsLink); err != nil {
		return err
	}

	buildAbsDir, err := d.toAbsoluteBuildDir(imageName, buildID)
	if err != nil {
		return err
	}
	return os.MkdirAll(buildAbsDir, 0o700)
}

// Clone clones source build to destination build
func (d *dirDriver) Clone(srcBuildID types.BuildID, dstImageName string, dstBuildID types.BuildID) error {
	dstBuildAbsDir, err := d.toAbsoluteBuildDir(dstImageName, dstBuildID)
	if err != nil {
		return err
	}

	srcBuildLink, err := d.toAbsoluteBuildLink(srcBuildID)
	if err != nil {
		return err
	}
	srcBuildDir, err := filepath.EvalSymlinks(srcBuildLink)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("build %s does not exist", srcBuildID)
		}
		return err
	}
	srcBuildAbsDir, err := filepath.Abs(srcBuildDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("build %s does not exist", srcBuildID)
		}
		return err
	}
	if err := d.CreateEmpty(dstImageName, dstBuildID); err != nil {
		return err
	}
	return copy.Copy(srcBuildAbsDir, dstBuildAbsDir, copy.Options{PreserveTimes: true, PreserveOwner: true})
}

// Manifest returns manifest of build
func (d *dirDriver) Manifest(buildID types.BuildID) (types.ImageManifest, error) {
	manifestFile, err := d.toAbsoluteManifestFile(buildID)
	if err != nil {
		return types.ImageManifest{}, err
	}
	manifestRaw, err := ioutil.ReadFile(manifestFile)
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
	manifestFile, err := d.toAbsoluteManifestFile(manifest.BuildID)
	if err != nil {
		return err
	}
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
	buildLink, err := d.toAbsoluteBuildLink(buildID)
	if err != nil {
		return err
	}
	buildDir, err := filepath.EvalSymlinks(buildLink)
	if err != nil {
		return err
	}
	buildAbsDir, err := filepath.Abs(buildDir)
	if err != nil {
		return err
	}
	parentDir := filepath.Dir(buildAbsDir)
	tagLink := filepath.Join(parentDir, string(tag))
retry:
	for {
		err := os.Symlink(string(buildID), tagLink)
		switch {
		case err == nil:
			break retry
		case os.IsExist(err):
			if err := os.Remove(tagLink); err != nil && !os.IsNotExist(err) {
				return err
			}
		default:
			return err
		}
	}
	return nil
}

// Drop drops image
func (d *dirDriver) Drop(buildID types.BuildID) (retErr error) {
	// Sequence is important:
	// 1. remove tags
	// 2. remove build dir
	// 3. remove build link

	buildLink, err := d.toAbsoluteBuildLink(buildID)
	if err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			return
		}
		if err := os.Remove(buildLink); err != nil && !os.IsNotExist(err) {
			retErr = err
		}
	}()

	buildDir, err := filepath.EvalSymlinks(buildLink)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	buildAbsDir, err := filepath.Abs(buildDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	tagsAbsDir := filepath.Dir(buildAbsDir)
	dir, err := os.Open(tagsAbsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer dir.Close()

	var entries []os.DirEntry
	for entries, err = dir.ReadDir(20); err == nil; entries, err = dir.ReadDir(20) {
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}

			if info.Mode()&os.ModeSymlink != 0 {
				tagAbsLink := filepath.Join(tagsAbsDir, info.Name())
				buildDirFromLink, err := filepath.EvalSymlinks(tagAbsLink)
				if err != nil {
					if os.IsNotExist(err) {
						// dead link, remove it
						if err := os.Remove(tagAbsLink); err != nil && !os.IsNotExist(err) {
							return err
						}
						continue
					}
					return err
				}
				buildAbsDirFromLink, err := filepath.Abs(buildDirFromLink)
				if err != nil {
					if os.IsNotExist(err) {
						// dead link, remove it
						if err := os.Remove(tagAbsLink); err != nil && !os.IsNotExist(err) {
							return err
						}
						continue
					}
					return err
				}
				if buildAbsDir == buildAbsDirFromLink {
					if err := os.Remove(tagAbsLink); err != nil && !os.IsNotExist(err) {
						return err
					}
				}
			}
		}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	manifestFile, err := d.toAbsoluteManifestFile(buildID)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.Remove(manifestFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.RemoveAll(buildDir)
}

func (d *dirDriver) toRelativeBuildDir(imageName string, buildID types.BuildID) string {
	return filepath.Join(subDirImages, imageName, string(buildID))
}

func (d *dirDriver) toAbsoluteBuildDir(imageName string, buildID types.BuildID) (string, error) {
	rootPath, err := filepath.Abs(d.rootPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, d.toRelativeBuildDir(imageName, buildID)), nil
}

func (d *dirDriver) toAbsoluteBuildLink(buildID types.BuildID) (string, error) {
	rootPath, err := filepath.Abs(d.rootPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, subDirBuilds, string(buildID)), nil
}

func (d *dirDriver) toAbsoluteTagLink(imageName string, tag types.Tag) (string, error) {
	rootPath, err := filepath.Abs(d.rootPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, subDirImages, imageName, string(tag)), nil
}

func (d *dirDriver) toAbsoluteManifestFile(buildID types.BuildID) (string, error) {
	rootPath, err := filepath.Abs(d.rootPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, subDirManifests, string(buildID)+".json"), nil
}

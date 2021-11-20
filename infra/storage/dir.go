package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/otiai10/copy"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/runtime"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
)

const (
	subDirImages = "images"
	subDirBuilds = "builds"
)

// NewDirDriver returns new storage driver based on directories
func NewDirDriver(config runtime.Config) Driver {
	return &dirDriver{
		rootPath: config.RootDir,
	}
}

type dirDriver struct {
	rootPath string
}

// Mount mounts image in filesystem
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

// CreateEmpty creates blank image for build
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

// Clone clones source image to destination or returns false if source image does not exist
func (d *dirDriver) Clone(srcImageName string, srcTag types.Tag, dstImageName string, dstBuildID types.BuildID) error {
	dstBuildAbsDir, err := d.toAbsoluteBuildDir(dstImageName, dstBuildID)
	if err != nil {
		return err
	}

	srcTagLink, err := d.toAbsoluteTagLink(srcImageName, srcTag)
	if err != nil {
		return err
	}
	srcBuildDir, err := filepath.EvalSymlinks(srcTagLink)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("image %s:%s does not exist: %w", srcImageName, srcTag, ErrSourceImageDoesNotExist)
		}
		return err
	}
	srcBuildAbsDir, err := filepath.Abs(srcBuildDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("image %s:%s does not exist: %w", srcImageName, srcTag, ErrSourceImageDoesNotExist)
		}
		return err
	}
	if err := d.CreateEmpty(dstImageName, dstBuildID); err != nil {
		return err
	}
	return copy.Copy(srcBuildAbsDir, dstBuildAbsDir, copy.Options{PreserveTimes: true, PreserveOwner: true})
}

// Tag tags buildID with tag
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

package osman

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/storage"
	"github.com/outofforest/osman/infra/types"
)

func copyKernel(buildMountPoint string, storage config.Storage, buildID types.BuildID) error {
	buildBootDir := filepath.Join(buildMountPoint, "boot")
	return forEachBootMaster(bootPrefix(storage.Root), func(diskMountpoint string) error {
		kernelDir := filepath.Join(diskMountpoint, "zfs", string(buildID))
		if err := os.MkdirAll(kernelDir, 0o755); err != nil {
			return errors.WithStack(err)
		}
		if err := copyFile(filepath.Join(kernelDir, "vmlinuz"), filepath.Join(buildBootDir, "vmlinuz"),
			0o755); err != nil {
			return errors.WithStack(err)
		}
		return errors.WithStack(copyFile(filepath.Join(kernelDir, "initramfs.img"),
			filepath.Join(buildBootDir, "initramfs.img"), 0o600))
	})
}

func cleanKernel(buildID types.BuildID, bootPrefix string) error {
	return forEachBootMaster(bootPrefix, func(diskMountpoint string) error {
		kernelDir := filepath.Join(diskMountpoint, "zfs", string(buildID))
		if err := os.RemoveAll(kernelDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.WithStack(err)
		}
		return nil
	})
}

//go:embed grub.tmpl.cfg
var grubTemplate string
var grubTemplateCompiled = template.Must(template.New("grub").Parse(grubTemplate))

type grubConfig struct {
	StorageRoot string
	Builds      []types.BuildInfo
}

func generateGRUB(ctx context.Context, storage config.Storage, s storage.Driver) error {
	builds, err := List(ctx, config.Filter{Types: []types.BuildType{types.BuildTypeBoot}}, s)
	if err != nil {
		return err
	}
	sort.Slice(builds, func(i, j int) bool {
		return builds[i].CreatedAt.After(builds[j].CreatedAt)
	})

	for i, b := range builds {
		if len(b.Tags) > 0 {
			builds[i].Name += ":" + string(b.Tags[0])
		}
	}

	config := grubConfig{
		StorageRoot: storage.Root,
		Builds:      builds,
	}
	buf := &bytes.Buffer{}
	if err := grubTemplateCompiled.Execute(buf, config); err != nil {
		return errors.WithStack(err)
	}
	grubConfig := buf.Bytes()
	return forEachBootMaster(bootPrefix(storage.Root), func(diskMountpoint string) error {
		grubDir := filepath.Join(diskMountpoint, "grub2")
		if err := os.WriteFile(filepath.Join(grubDir, "grub.cfg"), grubConfig, 0o644); err != nil {
			return errors.WithStack(err)
		}
		return errors.WithStack(os.WriteFile(filepath.Join(grubDir, fmt.Sprintf("grub-%s.cfg",
			time.Now().UTC().Format(time.RFC3339))), grubConfig, 0o644))
	})
}

func forEachBootMaster(prefix string, fn func(mountpoint string) error) error {
	path := "/dev/disk/by-label"
	files, err := os.ReadDir(path)
	if err != nil {
		return errors.WithStack(err)
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasPrefix(f.Name(), prefix) {
			continue
		}

		disk := filepath.Join(path, f.Name())
		diskMountpoint, err := os.MkdirTemp("", prefix+"*")
		if err != nil {
			return errors.WithStack(err)
		}
		if err := syscall.Mount(disk, diskMountpoint, "ext4", 0, ""); err != nil {
			return errors.Wrapf(err, "mounting disk '%s' failed", disk)
		}
		if err := fn(diskMountpoint); err != nil {
			return err
		}
		if err := syscall.Unmount(diskMountpoint, 0); err != nil {
			return errors.Wrapf(err, "unmounting disk '%s' failed", disk)
		}
		if err := os.Remove(diskMountpoint); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func copyFile(dst, src string, perm os.FileMode) error {
	//nolint:nosnakecase // imported constant
	srcFile, err := os.OpenFile(src, os.O_RDONLY, 0)
	if err != nil {
		return errors.WithStack(err)
	}
	defer srcFile.Close()

	//nolint:nosnakecase // imported constant
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, perm)
	if err != nil {
		return errors.WithStack(err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return errors.WithStack(err)
}

func bootPrefix(storageRoot string) string {
	return "boot-" + filepath.Base(storageRoot) + "-"
}

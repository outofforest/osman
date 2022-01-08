package osman

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra"
	"github.com/outofforest/osman/infra/storage"
	"github.com/outofforest/osman/infra/types"
	"github.com/ridge/must"
)

// Build builds image
func Build(ctx context.Context, build config.Build, s storage.Driver, builder *infra.Builder) ([]types.BuildInfo, error) {
	builds := make([]types.BuildInfo, 0, len(build.SpecFiles))
	for i, specFile := range build.SpecFiles {
		must.OK(os.Chdir(filepath.Dir(specFile)))
		buildID, err := builder.BuildFromFile(ctx, specFile, build.Names[i], build.Tags...)
		if err != nil {
			return nil, err
		}
		info, err := s.Info(buildID)
		if err != nil {
			return nil, err
		}
		builds = append(builds, info)
	}
	return builds, nil
}

// Mount mounts image
func Mount(mount config.Mount, s storage.Driver) (types.BuildInfo, error) {
	properties := mount.Type.Properties()
	if !properties.Mountable {
		panic(fmt.Errorf("non-mountable image type received: %s", mount.Type))
	}

	if !mount.ImageBuildID.IsValid() {
		var err error
		mount.ImageBuildID, err = s.BuildID(mount.ImageBuildKey)
		if err != nil {
			return types.BuildInfo{}, err
		}
	}
	if !mount.ImageBuildID.Type().Properties().Cloneable {
		return types.BuildInfo{}, fmt.Errorf("build %s is not cloneable", mount.ImageBuildID)
	}

	image, err := s.Info(mount.ImageBuildID)
	if err != nil {
		return types.BuildInfo{}, err
	}

	if mount.MountKey.Name == "" {
		mount.MountKey.Name = image.Name
	}

	if mount.MountKey.Tag == "" {
		mount.MountKey.Tag = types.Tag(types.RandomString(5))
	}

	if mount.VMFile == "" {
		mount.VMFile = filepath.Join(mount.XMLDir, mount.MountKey.Name+".xml")
	}

	var doc *etree.Document
	if properties.VM {
		doc = etree.NewDocument()
		if err := doc.ReadFromFile(mount.VMFile); err != nil {
			return types.BuildInfo{}, err
		}
		if mount.MountKey.Name == "" {
			mount.MountKey.Name = vmName(doc)
		}
	}

	if !mount.MountKey.IsValid() {
		return types.BuildInfo{}, fmt.Errorf("mount key %s is invalid", mount.MountKey)
	}

	if properties.VM {
		exists, err := vmExists(mount.MountKey, mount.LibvirtAddr)
		if err != nil {
			return types.BuildInfo{}, err
		}
		if exists {
			return types.BuildInfo{}, fmt.Errorf("vm %s has been already defined", mount.MountKey)
		}
	}

	info, err := cloneForMount(image, mount.MountKey, mount.Type, s)
	if err != nil {
		return types.BuildInfo{}, err
	}

	if properties.VM {
		prepareVM(doc, info, mount.MountKey)
		xmlDef, err := doc.WriteToString()
		if err != nil {
			return types.BuildInfo{}, err
		}
		if err := deployVM(xmlDef, mount.LibvirtAddr); err != nil {
			return types.BuildInfo{}, err
		}
	}
	return info, nil
}

// List lists builds
func List(filtering config.Filter, s storage.Driver) ([]types.BuildInfo, error) {
	buildTypes := map[types.BuildType]bool{}
	for _, buildType := range filtering.Types {
		buildTypes[buildType] = true
	}

	var buildIDs map[types.BuildID]bool
	if len(filtering.BuildIDs) > 0 {
		buildIDs = map[types.BuildID]bool{}
		for _, buildID := range filtering.BuildIDs {
			buildIDs[buildID] = true
		}
	}
	var buildKeys map[types.BuildKey]bool
	if len(filtering.BuildKeys) > 0 {
		buildKeys = map[types.BuildKey]bool{}
		for _, buildKey := range filtering.BuildKeys {
			buildKeys[buildKey] = true
		}
	}

	builds, err := s.Builds()
	if err != nil {
		return nil, err
	}
	list := make([]types.BuildInfo, 0, len(builds))
	for _, buildID := range builds {
		info, err := s.Info(buildID)
		if err != nil {
			return nil, err
		}

		if !listBuild(info, buildTypes, buildIDs, buildKeys, filtering.Untagged) {
			continue
		}
		list = append(list, info)
	}
	return list, nil
}

// Result contains error realted to build ID
type Result struct {
	BuildID types.BuildID
	Result  error
}

// Drop drops builds
func Drop(filtering config.Filter, drop config.Drop, s storage.Driver) ([]Result, error) {
	if !drop.All && len(filtering.BuildIDs) == 0 && len(filtering.BuildKeys) == 0 {
		return nil, errors.New("neither filters are provided nor All is set")
	}

	builds, err := List(filtering, s)
	if err != nil {
		return nil, err
	}

	toDelete := map[types.BuildID]types.BuildInfo{}
	tree := map[types.BuildID]types.BuildID{}
	for _, build := range builds {
		toDelete[build.BuildID] = build
		for {
			if _, exists := tree[build.BuildID]; exists {
				break
			}
			tree[build.BuildID] = build.BasedOn
			if build.BasedOn == "" {
				break
			}
			var err error
			build, err = s.Info(build.BuildID)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(toDelete) == 0 {
		return nil, fmt.Errorf("no builds were selected to delete")
	}

	enqueued := map[types.BuildID]bool{}
	deleteSequence := make([]types.BuildInfo, 0, len(builds))
	var sort func(buildID types.BuildID)
	sort = func(buildID types.BuildID) {
		if enqueued[buildID] {
			return
		}
		if baseBuildID := tree[buildID]; baseBuildID != "" {
			sort(baseBuildID)
		}
		if build, exists := toDelete[buildID]; exists {
			enqueued[buildID] = true
			deleteSequence = append(deleteSequence, build)
		}
	}
	for _, build := range builds {
		sort(build.BuildID)
	}

	results := make([]Result, 0, len(deleteSequence))
	for i := len(deleteSequence) - 1; i >= 0; i-- {
		build := deleteSequence[i]
		res := Result{BuildID: build.BuildID}
		if build.BuildID.Type().Properties().VM {
			res.Result = undeployVM(types.BuildKey{Name: build.Name, Tag: build.Tags[0]}, drop.LibvirtAddr)
		}
		if res.Result == nil {
			res.Result = s.Drop(build.BuildID)
		}
		results = append(results, res)
	}
	return results, nil
}

func listBuild(info types.BuildInfo, buildTypes map[types.BuildType]bool, buildIDs map[types.BuildID]bool, buildKeys map[types.BuildKey]bool, untagged bool) bool {
	if !buildTypes[info.BuildID.Type()] {
		return false
	}
	if untagged && len(info.Tags) > 0 {
		return false
	}
	if buildIDs != nil && buildIDs[info.BuildID] {
		return true
	}
	if buildKeys != nil {
		if buildKeys[types.NewBuildKey(info.Name, "")] {
			return true
		}
		for _, tag := range info.Tags {
			if buildKeys[types.NewBuildKey(info.Name, tag)] || buildKeys[types.NewBuildKey("", tag)] {
				return true
			}
		}
	}
	return buildIDs == nil && buildKeys == nil
}

func vmName(doc *etree.Document) string {
	nameTag := doc.FindElement("//name")
	if nameTag == nil {
		return ""
	}
	return nameTag.Text()
}

func cloneForMount(image types.BuildInfo, buildKey types.BuildKey, imageType types.BuildType, s storage.Driver) (types.BuildInfo, error) {
	buildID := types.NewBuildID(imageType)
	if _, _, err := s.Clone(image.BuildID, buildKey.Name, buildID); err != nil {
		return types.BuildInfo{}, err
	}
	if err := s.StoreManifest(types.ImageManifest{
		BuildID: buildID,
		BasedOn: image.BuildID,
		Params:  image.Params,
	}); err != nil {
		return types.BuildInfo{}, err
	}

	if err := s.Tag(buildID, buildKey.Tag); err != nil {
		return types.BuildInfo{}, err
	}

	return s.Info(buildID)
}

func prepareVM(doc *etree.Document, info types.BuildInfo, buildKey types.BuildKey) {
	nameTag := doc.FindElement("//name")
	for _, ch := range nameTag.Child {
		nameTag.RemoveChild(ch)
	}
	nameTag.CreateText(buildKey.String())

	devicesTag := doc.FindElement("//devices")
	filesystemTag := devicesTag.CreateElement("filesystem")
	filesystemTag.CreateAttr("type", "mount")
	filesystemTag.CreateAttr("accessmode", "passthrough")
	driverTag := filesystemTag.CreateElement("driver")
	driverTag.CreateAttr("type", "virtiofs")
	sourceTag := filesystemTag.CreateElement("source")
	sourceTag.CreateAttr("dir", info.Mounted+"/root")
	targetTag := filesystemTag.CreateElement("target")
	targetTag.CreateAttr("dir", "root")

	osTag := doc.FindElement("//os")
	kernelTag := osTag.FindElement("kernel")
	if kernelTag == nil {
		kernelTag = osTag.CreateElement("kernel")
	}
	for _, ch := range kernelTag.Child {
		kernelTag.RemoveChild(ch)
	}
	initrdTag := osTag.FindElement("initrd")
	if initrdTag == nil {
		initrdTag = osTag.CreateElement("initrd")
	}
	for _, ch := range initrdTag.Child {
		initrdTag.RemoveChild(ch)
	}
	cmdlineTag := osTag.FindElement("cmdline")
	if cmdlineTag == nil {
		cmdlineTag = osTag.CreateElement("cmdline")
	}
	for _, ch := range cmdlineTag.Child {
		cmdlineTag.RemoveChild(ch)
	}

	kernelTag.CreateText(info.Mounted + "/root/boot/vmlinuz")
	initrdTag.CreateText(info.Mounted + "/root/boot/initramfs.img")
	cmdlineTag.CreateText(strings.Join(append([]string{"root=virtiofs:root"}, info.Params...), " "))
}

func libvirtConn(addr string) (*libvirt.Libvirt, error) {
	addrParts := strings.SplitN(addr, "://", 2)
	if len(addrParts) != 2 {
		return nil, fmt.Errorf("address %s has invalid format", addr)
	}

	conn, err := net.DialTimeout(addrParts[0], addrParts[1], 2*time.Second)
	if err != nil {
		return nil, err
	}

	l := libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn))
	if err := l.Connect(); err != nil {
		return nil, err
	}
	return l, nil
}

func vmExists(buildKey types.BuildKey, libvirtAddr string) (bool, error) {
	l, err := libvirtConn(libvirtAddr)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = l.Disconnect()
	}()

	_, err = l.DomainLookupByName(buildKey.String())
	if libvirt.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func deployVM(vmDef string, libvirtAddr string) error {
	l, err := libvirtConn(libvirtAddr)
	if err != nil {
		return err
	}
	defer func() {
		_ = l.Disconnect()
	}()

	_, err = l.DomainDefineXML(vmDef)
	return err
}

func undeployVM(buildKey types.BuildKey, libvirtAddr string) error {
	l, err := libvirtConn(libvirtAddr)
	if err != nil {
		return err
	}
	defer func() {
		_ = l.Disconnect()
	}()

	domain, err := l.DomainLookupByName(buildKey.String())
	if libvirt.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := l.DomainUndefineFlags(domain, libvirt.DomainUndefineManagedSave|libvirt.DomainUndefineSnapshotsMetadata|libvirt.DomainUndefineNvram|libvirt.DomainUndefineCheckpointsMetadata); err != nil && !libvirt.IsNotFound(err) {
		return err
	}
	return nil
}

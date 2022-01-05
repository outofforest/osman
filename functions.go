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

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"

	"github.com/beevik/etree"
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
func Mount(mount config.Mount, s storage.Driver) error {
	if !mount.BuildID.IsValid() {
		var err error
		mount.BuildID, err = s.BuildID(mount.BuildKey)
		if err != nil {
			return err
		}
	}
	if !mount.BuildID.Type().Properties().Cloneable {
		return fmt.Errorf("build %s is not cloneable", mount.BuildID)
	}

	image, err := s.Info(mount.BuildID)
	if err != nil {
		return err
	}

	if mount.VMFile == "" {
		mount.VMFile = filepath.Join(mount.XMLDir, image.Name+".xml")
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromFile(mount.VMFile); err != nil {
		return err
	}

	name, err := vmName(doc)
	if err != nil {
		return err
	}

	list, err := List(config.Filter{
		Types: []types.BuildType{
			types.BuildTypeMount,
		},
		BuildKeys: []types.BuildKey{
			types.NewBuildKey(name, ""),
		},
	}, s)
	if err != nil {
		return err
	}
	if len(list) > 0 {
		return fmt.Errorf("build %s already exists", name)
	}

	info, err := cloneForMount(image, name, s)
	if err != nil {
		return err
	}

	prepareVM(doc, info, name)
	xmlDef, err := doc.WriteToString()
	if err != nil {
		return err
	}
	return deployVM(xmlDef, mount.LibvirtAddr)
}

func vmName(doc *etree.Document) (string, error) {
	nameTag := doc.FindElement("//name")
	if nameTag == nil {
		return "", errors.New("name tag is not present")
	}
	name := nameTag.Text()
	if name == "" {
		return "", errors.New("name is empty")
	}
	return name + "-" + types.RandomString(5), nil
}

func cloneForMount(image types.BuildInfo, name string, s storage.Driver) (types.BuildInfo, error) {
	buildID := types.NewBuildID(types.BuildTypeMount)
	if _, _, err := s.Clone(image.BuildID, name, buildID); err != nil {
		return types.BuildInfo{}, err
	}
	if err := s.StoreManifest(types.ImageManifest{
		BuildID: buildID,
		BasedOn: image.BuildID,
		Params:  image.Params,
	}); err != nil {
		return types.BuildInfo{}, err
	}

	return s.Info(buildID)
}

func prepareVM(doc *etree.Document, info types.BuildInfo, name string) {
	nameTag := doc.FindElement("//name")
	for _, ch := range nameTag.Child {
		nameTag.RemoveChild(ch)
	}
	nameTag.CreateText(name)

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
	if len(info.Params) > 0 {
		cmdlineTag.CreateText(strings.Join(info.Params, " "))
	}
}

func deployVM(vmDef string, libvirtAddr string) error {
	addrParts := strings.SplitN(libvirtAddr, "://", 2)
	if len(addrParts) != 2 {
		return fmt.Errorf("address %s has invalid format", libvirtAddr)
	}

	conn, err := net.DialTimeout(addrParts[0], addrParts[1], 2*time.Second)
	if err != nil {
		return err
	}

	l := libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn))
	if err := l.Connect(); err != nil {
		return err
	}
	defer func() {
		_ = l.Disconnect()
	}()

	_, err = l.DomainDefineXML(vmDef)
	return err
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

	toDelete := map[types.BuildID]bool{}
	tree := map[types.BuildID]types.BuildID{}
	for _, build := range builds {
		toDelete[build.BuildID] = true
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
	deleteSequence := make([]types.BuildID, 0, len(builds))
	var sort func(buildID types.BuildID)
	sort = func(buildID types.BuildID) {
		if enqueued[buildID] {
			return
		}
		if baseBuildID := tree[buildID]; baseBuildID != "" {
			sort(baseBuildID)
		}
		if toDelete[buildID] {
			enqueued[buildID] = true
			deleteSequence = append(deleteSequence, buildID)
		}
	}
	for _, build := range builds {
		sort(build.BuildID)
	}

	results := make([]Result, 0, len(deleteSequence))
	for i := len(deleteSequence) - 1; i >= 0; i-- {
		buildID := deleteSequence[i]
		results = append(results, Result{BuildID: buildID, Result: s.Drop(buildID)})
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

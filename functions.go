package osman

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/ridge/must"

	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra"
	"github.com/outofforest/osman/infra/storage"
	"github.com/outofforest/osman/infra/types"
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
		info, err := s.Info(ctx, buildID)
		if err != nil {
			return nil, err
		}
		builds = append(builds, info)
	}
	return builds, nil
}

// Mount mounts image
func Mount(ctx context.Context, mount config.Mount, s storage.Driver) (retInfo types.BuildInfo, retErr error) {
	properties := mount.Type.Properties()
	if !properties.Mountable {
		panic(errors.Errorf("non-mountable image type received: %s", mount.Type))
	}

	if !mount.ImageBuildID.IsValid() {
		var err error
		mount.ImageBuildID, err = s.BuildID(ctx, mount.ImageBuildKey)
		if err != nil {
			return types.BuildInfo{}, err
		}
	}
	if !mount.ImageBuildID.Type().Properties().Cloneable {
		return types.BuildInfo{}, errors.Errorf("build %s is not cloneable", mount.ImageBuildID)
	}

	image, err := s.Info(ctx, mount.ImageBuildID)
	if err != nil {
		return types.BuildInfo{}, err
	}

	var nameFromBuild bool
	if mount.MountKey.Name == "" {
		nameFromBuild = true
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
		if nameFromBuild {
			if name := vmName(doc); name != "" {
				mount.MountKey.Name = name
			}
		}
	}

	if !mount.MountKey.IsValid() {
		return types.BuildInfo{}, errors.Errorf("mount key %s is invalid", mount.MountKey)
	}

	if properties.VM {
		exists, err := vmExists(mount.MountKey, mount.LibvirtAddr)
		if err != nil {
			return types.BuildInfo{}, err
		}
		if exists {
			return types.BuildInfo{}, errors.Errorf("vm %s has been already defined", mount.MountKey)
		}
	}

	info, err := cloneForMount(ctx, image, mount.MountKey, mount.Type, s)
	if err != nil {
		return types.BuildInfo{}, err
	}
	defer func() {
		if retErr != nil {
			_ = s.Drop(ctx, info.BuildID)
		}
	}()

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
func List(ctx context.Context, filtering config.Filter, s storage.Driver) ([]types.BuildInfo, error) {
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

	builds, err := s.Builds(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]types.BuildInfo, 0, len(builds))
	for _, buildID := range builds {
		info, err := s.Info(ctx, buildID)
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
func Drop(ctx context.Context, filtering config.Filter, drop config.Drop, s storage.Driver) ([]Result, error) {
	if !drop.All && len(filtering.BuildIDs) == 0 && len(filtering.BuildKeys) == 0 {
		return nil, errors.New("neither filters are provided nor All is set")
	}

	builds, err := List(ctx, filtering, s)
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
			build, err = s.Info(ctx, build.BuildID)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(toDelete) == 0 {
		return nil, errors.New("no builds were selected to delete")
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
		res := Result{BuildID: buildID}
		if buildID.Type().Properties().VM {
			res.Result = undeployVM(buildID, drop.LibvirtAddr)
		}
		if res.Result == nil {
			res.Result = s.Drop(ctx, buildID)
		}
		results = append(results, res)
	}
	return results, nil
}

// Tag removes and add tags to the build
func Tag(ctx context.Context, filtering config.Filter, tag config.Tag, s storage.Driver) ([]types.BuildInfo, error) {
	if !tag.All && len(filtering.BuildIDs) == 0 && len(filtering.BuildKeys) == 0 {
		return nil, errors.New("neither filters are provided nor All is set")
	}

	builds, err := List(ctx, filtering, s)
	if err != nil {
		return nil, err
	}

	if len(builds) == 0 {
		return nil, errors.New("no builds were selected to tag")
	}

	for _, t := range tag.Remove {
		for _, build := range builds {
			if err := s.Untag(ctx, build.BuildID, t); err != nil {
				return nil, err
			}
		}
	}
	for _, t := range tag.Add {
		for _, build := range builds {
			if err := s.Tag(ctx, build.BuildID, t); err != nil {
				return nil, err
			}
		}
	}

	filtering = config.Filter{BuildIDs: make([]types.BuildID, 0, len(builds)), Types: filtering.Types}
	for _, b := range builds {
		filtering.BuildIDs = append(filtering.BuildIDs, b.BuildID)
	}
	return List(ctx, filtering, s)
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

func cloneForMount(ctx context.Context, image types.BuildInfo, buildKey types.BuildKey, imageType types.BuildType, s storage.Driver) (retInfo types.BuildInfo, retErr error) {
	buildID := types.NewBuildID(imageType)
	if _, _, err := s.Clone(ctx, image.BuildID, buildKey.Name, buildID); err != nil {
		return types.BuildInfo{}, err
	}
	defer func() {
		if retErr != nil {
			_ = s.Drop(ctx, buildID)
		}
	}()

	if err := s.StoreManifest(ctx, types.ImageManifest{
		BuildID: buildID,
		BasedOn: image.BuildID,
		Params:  image.Params,
	}); err != nil {
		return types.BuildInfo{}, err
	}

	if err := s.Tag(ctx, buildID, buildKey.Tag); err != nil {
		return types.BuildInfo{}, err
	}

	return s.Info(ctx, buildID)
}

func mac() string {
	buf := make([]byte, 5)
	must.Any(rand.Read(buf))
	res := "00" // just to ensure that unicast address is generated
	for _, b := range buf {
		res += fmt.Sprintf(":%02x", b)
	}
	return res
}

func prepareVM(doc *etree.Document, info types.BuildInfo, buildKey types.BuildKey) {
	nameTag := doc.FindElement("//name")
	for _, ch := range nameTag.Child {
		nameTag.RemoveChild(ch)
	}
	nameTag.CreateText(buildKey.String())

	domainTag := doc.Root()
	uuidTag := domainTag.FindElement("//uuid")
	if uuidTag == nil {
		uuidTag = domainTag.CreateElement("uuid")
	}
	for _, ch := range uuidTag.Child {
		uuidTag.RemoveChild(ch)
	}
	uuid, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	uuidTag.CreateText(uuid.String())

	metadataTag := domainTag.FindElement("//metadata")
	if metadataTag == nil {
		metadataTag = domainTag.CreateElement("metadata")
	}
	osmanTag := metadataTag.CreateElement("osman:osman")
	osmanTag.CreateAttr("xmlns:osman", "http://go.exw.co/osman")
	buildIDTag := osmanTag.CreateElement("osman:buildid")
	buildIDTag.CreateText(string(info.BuildID))

	devicesTag := doc.FindElement("//devices")

	for _, macTag := range devicesTag.FindElements("interface[@type='network']/mac") {
		addressAttr := macTag.SelectAttr("address")
		if addressAttr == nil {
			addressAttr = macTag.CreateAttr("address", "")
		}
		addressAttr.Value = mac()
	}

	filesystemTag := devicesTag.CreateElement("filesystem")
	filesystemTag.CreateAttr("type", "mount")
	filesystemTag.CreateAttr("accessmode", "passthrough")
	driverTag := filesystemTag.CreateElement("driver")
	driverTag.CreateAttr("type", "virtiofs")
	sourceTag := filesystemTag.CreateElement("source")
	sourceTag.CreateAttr("dir", info.Mounted)
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

	kernelTag.CreateText(info.Mounted + "/boot/vmlinuz")
	initrdTag.CreateText(info.Mounted + "/boot/initramfs.img")
	cmdlineTag.CreateText(strings.Join(append([]string{"root=virtiofs:root"}, info.Params...), " "))
}

func libvirtConn(addr string) (*libvirt.Libvirt, error) {
	addrParts := strings.SplitN(addr, "://", 2)
	if len(addrParts) != 2 {
		return nil, errors.Errorf("address %s has invalid format", addr)
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

func undeployVM(buildID types.BuildID, libvirtAddr string) error {
	l, err := libvirtConn(libvirtAddr)
	if err != nil {
		return err
	}
	defer func() {
		_ = l.Disconnect()
	}()

	domains, _, err := l.ConnectListAllDomains(1, libvirt.ConnectListDomainsActive|libvirt.ConnectListDomainsInactive)
	if err != nil {
		return err
	}
	for _, d := range domains {
		xml, err := l.DomainGetXMLDesc(d, 0)
		if err != nil {
			return err
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromString(xml); err != nil {
			return err
		}
		buildIDTag := doc.FindElement("//metadata/osman:osman/osman:buildid")
		if buildIDTag == nil {
			continue
		}
		if buildID == types.BuildID(buildIDTag.Text()) {
			if err := l.DomainUndefineFlags(d, libvirt.DomainUndefineManagedSave|libvirt.DomainUndefineSnapshotsMetadata|libvirt.DomainUndefineNvram|libvirt.DomainUndefineCheckpointsMetadata); err != nil && !libvirt.IsNotFound(err) {
				return err
			}
			return nil
		}
	}
	return nil
}

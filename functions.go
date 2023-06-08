package osman

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed" // to embed grub config template
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/beevik/etree"
	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/ridge/must"
	"libvirt.org/go/libvirtxml"

	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra"
	"github.com/outofforest/osman/infra/storage"
	"github.com/outofforest/osman/infra/types"
)

const (
	defaultNATInterface        = "osman-nat"
	defaultNATInterfaceNetwork = "10.0.0.0/24"
	hostInterface              = "bond0"
	virtualBridgePrefix        = "virbr"
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
func Mount(ctx context.Context, storage config.Storage, mount config.Mount, s storage.Driver) (retInfo types.BuildInfo, retErr error) {
	properties := mount.Type.Properties()
	if !properties.Mountable {
		return types.BuildInfo{}, errors.Errorf("non-mountable image type received: %s", mount.Type)
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

	if mount.Type == types.BuildTypeBoot && len(image.Boots) == 0 {
		return types.BuildInfo{}, errors.New("image can't be mounted for booting because it was built without specifying BOOT option(s)")
	}

	if mount.MountKey.Name == "" {
		mount.MountKey.Name = image.Name
	}

	if mount.MountKey.Tag == "" {
		mount.MountKey.Tag = types.Tag(types.RandomString(5))
	}

	if !mount.MountKey.IsValid() {
		return types.BuildInfo{}, errors.Errorf("mount key %s is invalid", mount.MountKey)
	}

	info, err := cloneForMount(ctx, image, storage, mount, s)
	if err != nil {
		return types.BuildInfo{}, err
	}
	defer func() {
		if retErr != nil {
			_ = s.Drop(ctx, info.BuildID)
		}
	}()

	return info, nil
}

// Start starts VM
func Start(ctx context.Context, storage config.Storage, start config.Start, s storage.Driver) (types.BuildInfo, error) {
	var nameFromBuild bool
	if start.MountKey.Name == "" {
		nameFromBuild = true
		start.MountKey.Name = start.ImageBuildKey.Name
	}
	if start.MountKey.Tag == "" {
		start.MountKey.Tag = types.Tag(types.RandomString(5))
	}
	if start.VMFile == "" {
		start.VMFile = filepath.Join(start.XMLDir, start.MountKey.Name+".xml")
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromFile(start.VMFile); err != nil {
		return types.BuildInfo{}, errors.WithStack(err)
	}

	if nameFromBuild {
		if name := vmName(doc); name != "" {
			start.MountKey.Name = name
		}
	}

	exists, err := vmExists(start.MountKey, start.LibvirtAddr)
	if err != nil {
		return types.BuildInfo{}, err
	}
	if exists {
		return types.BuildInfo{}, errors.Errorf("vm %s has been already defined", start.MountKey)
	}

	if err := ensureNetwork(ctx, start.LibvirtAddr); err != nil {
		return types.BuildInfo{}, err
	}

	info, err := Mount(ctx, storage, config.Mount{
		ImageBuildID:  start.ImageBuildID,
		ImageBuildKey: start.ImageBuildKey,
		MountKey:      start.MountKey,
		Type:          types.BuildTypeVM,
	}, s)
	if err != nil {
		return types.BuildInfo{}, err
	}

	prepareVM(doc, info, start.MountKey)
	xmlDef, err := doc.WriteToString()
	if err != nil {
		return types.BuildInfo{}, errors.WithStack(err)
	}
	if err := deployVM(xmlDef, start.LibvirtAddr); err != nil {
		return types.BuildInfo{}, err
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
func Drop(ctx context.Context, storage config.Storage, filtering config.Filter, drop config.Drop, s storage.Driver) ([]Result, error) {
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
	var genGRUB bool
	for i := len(deleteSequence) - 1; i >= 0; i-- {
		buildID := deleteSequence[i]
		res := Result{BuildID: buildID}
		if buildID.Type().Properties().VM {
			res.Result = undeployVM(buildID, drop.LibvirtAddr)
		}
		if res.Result == nil {
			res.Result = s.Drop(ctx, buildID)
		}
		if buildID.Type() == types.BuildTypeBoot && res.Result == nil {
			genGRUB = true
			res.Result = cleanKernel(buildID, "boot-")
		}

		results = append(results, res)
	}

	if genGRUB {
		if err := generateGRUB(ctx, storage, s); err != nil {
			return nil, err
		}
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

func cloneForMount(ctx context.Context, image types.BuildInfo, storage config.Storage, mount config.Mount, s storage.Driver) (retInfo types.BuildInfo, retErr error) {
	buildID := types.NewBuildID(mount.Type)
	finalizeFn, buildMountpoint, err := s.Clone(ctx, image.BuildID, mount.MountKey.Name, buildID)
	if err != nil {
		return types.BuildInfo{}, err
	}
	defer func() {
		if retErr != nil {
			_ = s.Drop(ctx, buildID)
		}
	}()

	manifest := types.ImageManifest{
		BuildID: buildID,
		BasedOn: image.BuildID,
		Params:  image.Params,
	}
	if mount.Type == types.BuildTypeBoot {
		manifest.Boots = image.Boots
	}

	if err := s.StoreManifest(ctx, manifest); err != nil {
		return types.BuildInfo{}, err
	}

	if err := s.Tag(ctx, buildID, mount.MountKey.Tag); err != nil {
		return types.BuildInfo{}, err
	}

	if mount.Type == types.BuildTypeBoot {
		if err := copyKernel(buildMountpoint, storage, buildID); err != nil {
			return types.BuildInfo{}, err
		}
	}

	if err := finalizeFn(); err != nil {
		return types.BuildInfo{}, err
	}

	if mount.Type == types.BuildTypeBoot {
		if err := generateGRUB(ctx, storage, s); err != nil {
			return types.BuildInfo{}, err
		}
	}

	return s.Info(ctx, buildID)
}

func copyKernel(buildMountPoint string, storage config.Storage, buildID types.BuildID) error {
	buildBinDir := filepath.Join(buildMountPoint, "boot")
	return forEachBootMaster(bootPrefix(storage.Root), func(diskMountpoint string) error {
		kernelDir := filepath.Join(diskMountpoint, "zfs", string(buildID))
		if err := os.MkdirAll(kernelDir, 0o755); err != nil {
			return errors.WithStack(err)
		}
		if err := copyFile(filepath.Join(kernelDir, "vmlinuz"), filepath.Join(buildBinDir, "vmlinuz"), 0o755); err != nil {
			return errors.WithStack(err)
		}
		return errors.WithStack(copyFile(filepath.Join(kernelDir, "initramfs.img"), filepath.Join(buildBinDir, "initramfs.img"), 0o600))
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
		return errors.WithStack(os.WriteFile(filepath.Join(grubDir, fmt.Sprintf("grub-%s.cfg", time.Now().UTC().Format(time.RFC3339))), grubConfig, 0o644))
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

func mac() string {
	buf := make([]byte, 5)
	must.Any(rand.Read(buf))
	res := "00" // to ensure that unicast address is generated
	for _, b := range buf {
		res += fmt.Sprintf(":%02x", b)
	}
	return res
}

func ensureNetwork(ctx context.Context, libvirtAddr string) error {
	l, err := libvirtConn(libvirtAddr)
	if err != nil {
		return err
	}
	defer func() {
		_ = l.Disconnect()
	}()

	_, err = l.NetworkLookupByName(defaultNATInterface)
	if err == nil {
		return nil
	}
	if !isError(err, libvirt.ErrNoNetwork) {
		return errors.WithStack(err)
	}

	_, ipNet, err := net.ParseCIDR(defaultNATInterfaceNetwork)
	if err != nil {
		return errors.WithStack(err)
	}

	ones, bits := ipNet.Mask.Size()
	netSize := uint32(1 << (bits - ones))

	bridgeSuffix := sha256.Sum256([]byte(defaultNATInterface))
	network, err := l.NetworkDefineXML(must.String((&libvirtxml.Network{
		Name: defaultNATInterface,
		Forward: &libvirtxml.NetworkForward{
			Mode: "nat",
			Dev:  hostInterface,
			Interfaces: []libvirtxml.NetworkForwardInterface{
				{
					Dev: hostInterface,
				},
			},
		},
		Bridge: &libvirtxml.NetworkBridge{
			Name:  virtualBridgePrefix + hex.EncodeToString(bridgeSuffix[:])[:15-len(virtualBridgePrefix)],
			STP:   "on",
			Delay: "0",
		},
		IPs: []libvirtxml.NetworkIP{
			{
				Address: uint32ToIP4(ip4ToUint32(ipNet.IP) + 1).To4().String(),
				Netmask: fmt.Sprintf("%d.%d.%d.%d", ipNet.Mask[0], ipNet.Mask[1], ipNet.Mask[2], ipNet.Mask[3]),
				DHCP: &libvirtxml.NetworkDHCP{Ranges: []libvirtxml.NetworkDHCPRange{
					{
						Start: uint32ToIP4(ip4ToUint32(ipNet.IP) + 2).To4().String(),
						End:   uint32ToIP4(ip4ToUint32(ipNet.IP) + netSize - 2).To4().String(),
					},
				}},
			},
		},
	}).Marshal()))
	if err != nil {
		return errors.WithStack(err)
	}
	if err := l.NetworkCreate(network); err != nil {
		return errors.WithStack(err)
	}

	for {
		network, err := l.NetworkLookupByName(defaultNATInterface)
		if err != nil {
			return errors.WithStack(err)
		}
		active, err := l.NetworkIsActive(network)
		if err != nil {
			return errors.WithStack(err)
		}
		if active == 1 {
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.WithStack(err)
		case <-time.After(100 * time.Millisecond):
		}
	}
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
	binaryTag := filesystemTag.CreateElement("binary")
	binaryTag.CreateAttr("path", "/usr/libexec/virtiofsd")
	threadPoolTag := binaryTag.CreateElement("thread_pool")
	threadPoolTag.CreateAttr("size", "1")
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
		return nil, errors.WithStack(err)
	}

	l := libvirt.NewWithDialer(dialers.NewAlreadyConnected(conn))
	if err := l.Connect(); err != nil {
		return nil, errors.WithStack(err)
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

	domain, err := l.DomainDefineXML(vmDef)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := l.DomainCreate(domain); err != nil {
		return errors.WithStack(err)
	}

	return nil
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
		return errors.WithStack(err)
	}
	for _, d := range domains {
		xml, err := l.DomainGetXMLDesc(d, 0)
		if err != nil {
			return errors.WithStack(err)
		}
		doc := etree.NewDocument()
		if err := doc.ReadFromString(xml); err != nil {
			return errors.WithStack(err)
		}
		buildIDTag := doc.FindElement("//metadata/osman:osman/osman:buildid")
		if buildIDTag == nil {
			continue
		}
		if buildID == types.BuildID(buildIDTag.Text()) {
			if err := l.DomainUndefineFlags(d, libvirt.DomainUndefineManagedSave|libvirt.DomainUndefineSnapshotsMetadata|libvirt.DomainUndefineNvram|libvirt.DomainUndefineCheckpointsMetadata); err != nil && !libvirt.IsNotFound(err) {
				return errors.WithStack(err)
			}
			return nil
		}
	}
	return nil
}

func bootPrefix(storageRoot string) string {
	return "boot-" + filepath.Base(storageRoot) + "-"
}

func ip4ToUint32(ip net.IP) uint32 {
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func uint32ToIP4(val uint32) net.IP {
	return net.IPv4(byte(val>>24), byte(val>>16), byte(val>>8), byte(val)).To4()
}

func isError(err error, expectedError libvirt.ErrorNumber) bool {
	for err != nil {
		e, ok := err.(libvirt.Error)
		if ok {
			return e.Code == uint32(expectedError)
		}
		err = errors.Unwrap(err)
	}
	return false
}

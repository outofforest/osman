package osman

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/beevik/etree"
	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/ridge/must"
	"github.com/vishvananda/netlink"
	"libvirt.org/go/libvirtxml"

	"github.com/outofforest/osman/infra/types"
)

const (
	defaultNATNetworkName = "osman-nat"
	defaultNATNetwork     = "10.0.0.0/24"
	virtualBridgePrefix   = "virbr"
)

func mac() string {
	buf := make([]byte, 5)
	must.Any(rand.Read(buf))
	res := "00" // to ensure that unicast address is generated
	for _, b := range buf {
		res += fmt.Sprintf(":%02x", b)
	}
	return res
}

func addVMToDefaultNetwork(ctx context.Context, l *libvirt.Libvirt, mac string) error {
	_, ipNet, err := net.ParseCIDR(defaultNATNetwork)
	if err != nil {
		return errors.WithStack(err)
	}
	netSize := netSize(ipNet)

	network, err := l.NetworkLookupByName(defaultNATNetworkName)
	if err != nil {
		if !isError(err, libvirt.ErrNoNetwork) {
			return errors.WithStack(err)
		}

		routes, err := netlink.RouteList(nil, syscall.AF_UNSPEC)
		if err != nil {
			return errors.WithStack(err)
		}

		var defaultIface *net.Interface
		for _, r := range routes {
			if isDefaultRoute(r) {
				var err error
				if defaultIface, err = net.InterfaceByIndex(r.LinkIndex); err != nil {
					return errors.WithStack(err)
				}
				break
			}
		}
		if defaultIface == nil {
			return errors.New("default network interface not found")
		}

		bridgeSuffix := sha256.Sum256([]byte(defaultNATNetworkName))
		network, err = l.NetworkDefineXML(must.String((&libvirtxml.Network{
			Name: defaultNATNetworkName,
			Forward: &libvirtxml.NetworkForward{
				Mode: "nat",
				Dev:  defaultIface.Name,
				Interfaces: []libvirtxml.NetworkForwardInterface{
					{
						Dev: defaultIface.Name,
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
			network, err := l.NetworkLookupByName(defaultNATNetworkName)
			if err != nil {
				return errors.WithStack(err)
			}
			active, err := l.NetworkIsActive(network)
			if err != nil {
				return errors.WithStack(err)
			}
			if active == 1 {
				break
			}

			select {
			case <-ctx.Done():
				return errors.WithStack(err)
			case <-time.After(100 * time.Millisecond):
			}
		}
	}

	networkXML, err := l.NetworkGetXMLDesc(network, 0)
	if err != nil {
		return errors.WithStack(err)
	}

	var networkDoc libvirtxml.Network
	if err := networkDoc.Unmarshal(networkXML); err != nil {
		return err
	}
	if len(networkDoc.IPs) == 0 {
		return errors.Errorf("no IP section defined on network %q", defaultNATNetworkName)
	}
	if len(networkDoc.IPs) > 1 {
		return errors.Errorf("exactly one IP section is expected for network on network %q", defaultNATNetworkName)
	}

	usedIPs := map[uint32]struct{}{}
	if ip := networkDoc.IPs[0]; ip.DHCP != nil {
		for _, host := range ip.DHCP.Hosts {
			if host.MAC == mac {
				return errors.Errorf("mac address %q already defined on network %q", mac, defaultNATNetworkName)
			}
			usedIPs[ip4ToUint32(net.ParseIP(host.IP))] = struct{}{}
		}
	}

	start := ip4ToUint32(ipNet.IP) + 2
	end := start + netSize - 3
	for i := start; i <= end; i++ {
		if _, exists := usedIPs[i]; exists {
			continue
		}

		err := l.NetworkUpdateCompat(
			network,
			libvirt.NetworkUpdateCommandAddLast,
			libvirt.NetworkSectionIPDhcpHost,
			0,
			fmt.Sprintf("<host mac='%s' ip='%s' />", mac, uint32ToIP4(i)),
			libvirt.NetworkUpdateAffectLive|libvirt.NetworkUpdateAffectConfig,
		)
		if err != nil {
			return errors.WithStack(err)
		}

		return nil
	}

	return errors.Errorf("no free IPs in the network %q", defaultNATNetworkName)
}

func removeVMFromDefaultNetwork(l *libvirt.Libvirt, mac string) error {
	network, err := l.NetworkLookupByName(defaultNATNetworkName)
	if isError(err, libvirt.ErrNoNetwork) {
		return nil
	}
	if err != nil {
		return errors.WithStack(err)
	}

	networkXML, err := l.NetworkGetXMLDesc(network, 0)
	if err != nil {
		return errors.WithStack(err)
	}

	var networkDoc libvirtxml.Network
	if err := networkDoc.Unmarshal(networkXML); err != nil {
		return errors.WithStack(err)
	}

	if len(networkDoc.IPs) == 0 {
		return errors.Errorf("no IP section defined on network %q", defaultNATNetworkName)
	}
	if len(networkDoc.IPs) > 1 {
		return errors.Errorf("exactly one IP section is expected for network on network %q", defaultNATNetworkName)
	}

	var ipToDelete string
	networkNeeded := false

	if ip := networkDoc.IPs[0]; ip.DHCP != nil {
		for _, h := range ip.DHCP.Hosts {
			if h.MAC == mac {
				ipToDelete = h.IP
			} else {
				networkNeeded = true
			}
		}
	}

	switch {
	case !networkNeeded:
		if err := l.NetworkDestroy(network); err != nil {
			return errors.WithStack(err)
		}
		if err := l.NetworkUndefine(network); err != nil {
			return errors.WithStack(err)
		}
	case ipToDelete != "":
		err := l.NetworkUpdateCompat(
			network,
			libvirt.NetworkUpdateCommandDelete,
			libvirt.NetworkSectionIPDhcpHost,
			0,
			fmt.Sprintf("<host mac='%s' ip='%s' />", mac, ipToDelete),
			libvirt.NetworkUpdateAffectLive|libvirt.NetworkUpdateAffectConfig,
		)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func prepareVM(l *libvirt.Libvirt, domainDoc libvirtxml.Domain, info types.BuildInfo, buildKey types.BuildKey) (libvirtxml.Domain, string, error) {
	mac := mac()
	domainDoc.Name = buildKey.String()

	uuid, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	domainDoc.UUID = uuid.String()

	domainDoc.Metadata = &libvirtxml.DomainMetadata{
		XML: fmt.Sprintf(`<osman:osman xmlns:osman="http://go.exw.co/osman"><osman:buildid>%s</osman:buildid></osman:osman>`, info.BuildID),
	}

	if domainDoc.Devices == nil {
		domainDoc.Devices = &libvirtxml.DomainDeviceList{}
	}
	domainDoc.Devices.Interfaces = append(domainDoc.Devices.Interfaces, libvirtxml.DomainInterface{
		MAC: &libvirtxml.DomainInterfaceMAC{
			Address: mac,
		},
		Source: &libvirtxml.DomainInterfaceSource{
			Network: &libvirtxml.DomainInterfaceSourceNetwork{
				Network: defaultNATNetworkName,
			},
		},
		Model: &libvirtxml.DomainInterfaceModel{
			Type: "virtio",
		},
	})

	domainDoc.Devices.Filesystems = append(domainDoc.Devices.Filesystems, libvirtxml.DomainFilesystem{
		Driver: &libvirtxml.DomainFilesystemDriver{
			Type: "virtiofs",
		},
		Binary: &libvirtxml.DomainFilesystemBinary{
			Path: "/usr/libexec/virtiofsd",
			ThreadPool: &libvirtxml.DomainFilesystemBinaryThreadPool{
				Size: 1,
			},
		},
		Source: &libvirtxml.DomainFilesystemSource{
			Mount: &libvirtxml.DomainFilesystemSourceMount{
				Dir: info.Mounted,
			},
		},
		Target: &libvirtxml.DomainFilesystemTarget{
			Dir: "root",
		},
	})

	if domainDoc.OS == nil {
		domainDoc.OS = &libvirtxml.DomainOS{}
	}
	domainDoc.OS.Kernel = info.Mounted + "/boot/vmlinuz"
	domainDoc.OS.Initrd = info.Mounted + "/boot/initramfs.img"
	domainDoc.OS.Cmdline = strings.Join(append([]string{"root=virtiofs:root"}, info.Params...), " ")

	if domainDoc.CPU == nil || domainDoc.CPU.Topology == nil {
		return libvirtxml.Domain{}, "", errors.New("cpu topology must be specified")
	}

	capabilitiesRaw, err := l.ConnectGetCapabilities()
	if err != nil {
		return libvirtxml.Domain{}, "", errors.WithStack(err)
	}

	var capabilitiesDoc libvirtxml.Caps
	if err := capabilitiesDoc.Unmarshal(capabilitiesRaw); err != nil {
		return libvirtxml.Domain{}, "", errors.WithStack(err)
	}

	if domainDoc.VCPU == nil || domainDoc.VCPU.Value == 0 {
		return libvirtxml.Domain{}, "", errors.New("number of vcpus is not provided")
	}
	if domainDoc.CPU == nil {
		return libvirtxml.Domain{}, "", errors.New("cpu settings are not provided")
	}
	domainDoc.VCPU.Value += domainDoc.VCPU.Value % uint(capabilitiesDoc.Host.CPU.Topology.Threads)
	cores := int(domainDoc.VCPU.Value) / capabilitiesDoc.Host.CPU.Topology.Threads
	domainDoc.CPU.Topology = &libvirtxml.DomainCPUTopology{
		Sockets: 1,
		Dies:    1,
		Cores:   cores,
		Threads: capabilitiesDoc.Host.CPU.Topology.Threads,
	}

	if domainDoc.IOThreads == 0 {
		domainDoc.IOThreads = 1
	}

	availableVCPUs, err := computeVCPUAvailability(l)
	if err != nil {
		return libvirtxml.Domain{}, "", errors.WithStack(err)
	}
	if len(availableVCPUs) < cores {
		return libvirtxml.Domain{}, "", errors.New("vm requires more cores than available on host")
	}

	domainDoc.CPUTune = &libvirtxml.DomainCPUTune{}
	var vcpuIndex uint
	i := 0
	for ; i < cores; i++ {
		for _, cpuID := range availableVCPUs[i] {
			domainDoc.CPUTune.VCPUPin = append(domainDoc.CPUTune.VCPUPin, libvirtxml.DomainCPUTuneVCPUPin{
				VCPU:   vcpuIndex,
				CPUSet: fmt.Sprintf("%d", cpuID),
			})
			vcpuIndex++
		}
	}
	domainDoc.CPUTune.IOThreadPin = []libvirtxml.DomainCPUTuneIOThreadPin{}
	for j := uint(1); j <= domainDoc.IOThreads; i, j = i+1, j+1 {
		if i == len(availableVCPUs) {
			i = 0
		}
		domainDoc.CPUTune.IOThreadPin = append(domainDoc.CPUTune.IOThreadPin, libvirtxml.DomainCPUTuneIOThreadPin{
			IOThread: j,
			CPUSet:   joinUInts(availableVCPUs[i]),
		})
	}
	if i == len(availableVCPUs) {
		i = 0
	}
	domainDoc.CPUTune.EmulatorPin = &libvirtxml.DomainCPUTuneEmulatorPin{
		CPUSet: joinUInts(availableVCPUs[i]),
	}

	return domainDoc, mac, nil
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

func vmExists(l *libvirt.Libvirt, buildKey types.BuildKey) (bool, error) {
	_, err := l.DomainLookupByName(buildKey.String())
	if libvirt.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func deployVM(l *libvirt.Libvirt, domainDoc libvirtxml.Domain) error {
	xml, err := domainDoc.Marshal()
	if err != nil {
		return errors.WithStack(err)
	}
	domain, err := l.DomainDefineXML(xml)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := l.DomainCreate(domain); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func undeployVM(l *libvirt.Libvirt, buildID types.BuildID) error {
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
			var domainDoc libvirtxml.Domain
			if err := domainDoc.Unmarshal(xml); err != nil {
				return errors.WithStack(err)
			}

			var mac string
			if domainDoc.Devices != nil {
				for _, i := range domainDoc.Devices.Interfaces {
					if i.Source == nil || i.Source.Network == nil {
						continue
					}
					if i.Source.Network.Network == defaultNATNetworkName && i.MAC != nil {
						mac = i.MAC.Address
					}
				}
			}

			if err := l.DomainUndefineFlags(d, libvirt.DomainUndefineManagedSave|libvirt.DomainUndefineSnapshotsMetadata|libvirt.DomainUndefineNvram|libvirt.DomainUndefineCheckpointsMetadata); err != nil && !libvirt.IsNotFound(err) {
				return errors.WithStack(err)
			}

			if mac != "" {
				return removeVMFromDefaultNetwork(l, mac)
			}

			return nil
		}
	}
	return nil
}

func ip4ToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func uint32ToIP4(val uint32) net.IP {
	return net.IPv4(byte(val>>24), byte(val>>16), byte(val>>8), byte(val)).To4()
}

func netSize(ipNet *net.IPNet) uint32 {
	ones, bits := ipNet.Mask.Size()
	return uint32(1 << (bits - ones))
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

type sibling struct {
	Weight uint
	CPUs   []uint
}
type socket struct {
	Weight         uint
	CPUToSiblings  map[uint]string
	Siblings       map[string]*sibling
	SiblingsToSort []*sibling
}

func computeVCPUAvailability(l *libvirt.Libvirt) ([][]uint, error) {
	capabilitiesRaw, err := l.ConnectGetCapabilities()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var capabilitiesDoc libvirtxml.Caps
	if err := capabilitiesDoc.Unmarshal(capabilitiesRaw); err != nil {
		return nil, errors.WithStack(err)
	}

	sockets := map[uint]*socket{}
	socketsToSort := []*socket{}
	cpuToSockets := map[uint]*socket{}
	for _, cell := range capabilitiesDoc.Host.NUMA.Cells.Cells {
		for _, cpu := range cell.CPUS.CPUs {
			cpuID := uint(cpu.ID)
			socketID := uint(*cpu.SocketID)
			sck, exists := sockets[socketID]
			if !exists {
				sck = &socket{
					CPUToSiblings: map[uint]string{},
					Siblings:      map[string]*sibling{},
				}
				sockets[socketID] = sck
				socketsToSort = append(socketsToSort, sck)
			}
			cpuToSockets[cpuID] = sck
			sck.CPUToSiblings[cpuID] = cpu.Siblings
			sbl, exists := sck.Siblings[cpu.Siblings]
			if !exists {
				sbl = &sibling{}
				sck.Siblings[cpu.Siblings] = sbl
				sck.SiblingsToSort = append(sck.SiblingsToSort, sbl)
			}
			sbl.CPUs = append(sbl.CPUs, cpuID)
		}
	}

	domains, _, err := l.ConnectListAllDomains(1, libvirt.ConnectListDomainsActive|libvirt.ConnectListDomainsInactive)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, d := range domains {
		xml, err := l.DomainGetXMLDesc(d, 0)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		var domainDoc libvirtxml.Domain
		if err := domainDoc.Unmarshal(xml); err != nil {
			return nil, errors.WithStack(err)
		}

		if domainDoc.CPUTune == nil {
			continue
		}
		cpuSet := []string{}
		for _, pin := range domainDoc.CPUTune.VCPUPin {
			cpuSet = append(cpuSet, strings.Split(pin.CPUSet, ",")...)
		}
		for _, pin := range domainDoc.CPUTune.IOThreadPin {
			cpuSet = append(cpuSet, strings.Split(pin.CPUSet, ",")...)
		}
		if domainDoc.CPUTune.EmulatorPin != nil {
			cpuSet = append(cpuSet, strings.Split(domainDoc.CPUTune.EmulatorPin.CPUSet, ",")...)
		}
		for _, cpuStr := range cpuSet {
			cpuStr = strings.TrimSpace(cpuStr)
			if cpuStr == "" {
				continue
			}
			cpu, err := strconv.Atoi(cpuStr)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			cpuID := uint(cpu)
			sck := cpuToSockets[cpuID]
			sck.Weight++
			sck.Siblings[sck.CPUToSiblings[cpuID]].Weight++
		}
	}

	sort.Slice(socketsToSort, func(i, j int) bool {
		return socketsToSort[i].Weight < socketsToSort[j].Weight
	})
	vcpus := [][]uint{}
	for _, sck := range socketsToSort {
		//nolint:scopelint // using sck in function below is fine because code is sequential
		sort.Slice(sck.SiblingsToSort, func(i, j int) bool {
			return sck.SiblingsToSort[i].Weight < sck.SiblingsToSort[j].Weight
		})
		for _, sbl := range sck.SiblingsToSort {
			vcpus = append(vcpus, sbl.CPUs)
		}
	}

	return vcpus, nil
}

func isDefaultRoute(route netlink.Route) bool {
	if route.Dst == nil {
		return true
	}
	ones, _ := route.Dst.Mask.Size()
	return ones == 0
}

func joinUInts(vals []uint) string {
	result := ""
	for _, v := range vals {
		if result != "" {
			result += ","
		}
		result += fmt.Sprintf("%d", v)
	}
	return result
}

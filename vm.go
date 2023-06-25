package osman

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/beevik/etree"
	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/ridge/must"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"libvirt.org/go/libvirtxml"

	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/types"
)

const tableNAT = "nat"

type network struct {
	Name    string
	Type    string
	Network string
	Bridge  string
}

var networkNAT = network{
	Name:    "osman",
	Type:    "open",
	Network: "10.0.0.0/24",
	Bridge:  "osman",
}

func addNetworkToFirewall(n network) error {
	defaultIfaceName, err := defaultIface()
	if err != nil {
		return err
	}

	c := &nftables.Conn{}
	chains, err := c.ListChains()
	if err != nil {
		return errors.WithStack(err)
	}

	var postroutingChain *nftables.Chain
	for _, ch := range chains {
		if ch.Table != nil &&
			ch.Table.Family == nftables.TableFamilyIPv4 &&
			ch.Type == nftables.ChainTypeNAT &&
			ch.Name == "POSTROUTING" {
			postroutingChain = ch
			break
		}
	}

	if postroutingChain == nil {
		var natTable *nftables.Table
		tables, err := c.ListTables()
		if err != nil {
			return errors.WithStack(err)
		}

		for _, t := range tables {
			if t.Family == nftables.TableFamilyIPv4 &&
				t.Name == tableNAT {
				natTable = t
				break
			}
		}

		if natTable == nil {
			return errors.New("no nat table")
		}

		postroutingChain = c.AddChain(&nftables.Chain{
			Name:     "POSTROUTING",
			Table:    natTable,
			Type:     nftables.ChainTypeNAT,
			Hooknum:  nftables.ChainHookPostrouting,
			Priority: nftables.ChainPriorityNATSource,
		})
	}

	osmanPostroutingChain := c.AddChain(&nftables.Chain{
		Name:  "OSMAN_POSTROUTING",
		Table: postroutingChain.Table,
	})
	c.AddRule(&nftables.Rule{
		Table: postroutingChain.Table,
		Chain: postroutingChain,
		Exprs: []expr.Any{
			&expr.Counter{},
			&expr.Verdict{
				Kind:  expr.VerdictJump,
				Chain: osmanPostroutingChain.Name,
			},
		},
	})
	c.AddRule(&nftables.Rule{
		Table: osmanPostroutingChain.Table,
		Chain: osmanPostroutingChain,
		Exprs: []expr.Any{
			&expr.Meta{Key: expr.MetaKeyIIFNAME, Register: 1},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte(n.Bridge + "\x00"),
			},
			&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     []byte(defaultIfaceName + "\x00"),
			},
			&expr.Counter{},
			&expr.Masq{},
		},
	})

	return errors.WithStack(c.Flush())
}

func ensureNetwork(ctx context.Context, l *libvirt.Libvirt, n network) error {
	_, err := l.NetworkLookupByName(n.Name)
	if err == nil {
		return nil
	}
	if !isError(err, libvirt.ErrNoNetwork) {
		return errors.WithStack(err)
	}

	_, netIP, err := net.ParseCIDR(n.Network)
	if err != nil {
		return errors.WithStack(err)
	}

	start := ip4ToUint32(netIP.IP) + 2
	end := start + netSize(netIP) - 4
	hosts := make([]libvirtxml.NetworkDHCPHost, 0, end-start+1)
	for i := start; i <= end; i++ {
		ip := uint32ToIP4(i)
		hosts = append(hosts, libvirtxml.NetworkDHCPHost{
			IP:  ip.String(),
			MAC: ipToMAC(ip),
		})
	}

	networkDoc := &libvirtxml.Network{
		Name: n.Name,
		Forward: &libvirtxml.NetworkForward{
			Mode: n.Type,
		},
		Bridge: &libvirtxml.NetworkBridge{
			Name:  n.Bridge,
			STP:   "on",
			Delay: "0",
		},
		IPs: []libvirtxml.NetworkIP{
			{
				Address: uint32ToIP4(ip4ToUint32(netIP.IP) + 1).To4().String(),
				Netmask: fmt.Sprintf("%d.%d.%d.%d", netIP.Mask[0], netIP.Mask[1], netIP.Mask[2], netIP.Mask[3]),
				DHCP: &libvirtxml.NetworkDHCP{
					Hosts: hosts,
				},
			},
		},
	}
	network, err := l.NetworkDefineXML(must.String(networkDoc.Marshal()))
	if err != nil {
		return errors.WithStack(err)
	}
	if err := l.NetworkCreate(network); err != nil {
		return errors.WithStack(err)
	}

	for {
		active, err := l.NetworkIsActive(network)
		if err != nil {
			return errors.WithStack(err)
		}
		if active == 1 {
			break
		}

		select {
		case <-ctx.Done():
			return errors.WithStack(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}

	return addNetworkToFirewall(n)
}

func findAvailableIPAndMAC(l *libvirt.Libvirt, n network) (net.IP, string, error) {
	network, err := l.NetworkLookupByName(n.Name)
	if isError(err, libvirt.ErrNoNetwork) {
		_, netIP, err := net.ParseCIDR(n.Network)
		if err != nil {
			return nil, "", errors.WithStack(err)
		}
		ip := uint32ToIP4(ip4ToUint32(netIP.IP) + 2)
		return ip, ipToMAC(ip), nil
	}
	if err != nil {
		return nil, "", errors.WithStack(err)
	}

	usedMACs := map[string]struct{}{}
	domains, _, err := l.ConnectListAllDomains(1, libvirt.ConnectListDomainsActive|libvirt.ConnectListDomainsInactive)
	if err != nil {
		return nil, "", errors.WithStack(err)
	}
	for _, d := range domains {
		xml, err := l.DomainGetXMLDesc(d, 0)
		if err != nil {
			return nil, "", errors.WithStack(err)
		}

		var domainDoc libvirtxml.Domain
		if err := domainDoc.Unmarshal(xml); err != nil {
			return nil, "", errors.WithStack(err)
		}

		if domainDoc.Devices == nil {
			continue
		}

		for _, iface := range domainDoc.Devices.Interfaces {
			if iface.Source == nil || iface.Source.Network == nil || iface.Source.Network.Network != n.Name ||
				iface.MAC == nil || iface.MAC.Address == "" {
				continue
			}
			usedMACs[iface.MAC.Address] = struct{}{}
		}
	}

	networkXML, err := l.NetworkGetXMLDesc(network, 0)
	if err != nil {
		return nil, "", errors.WithStack(err)
	}

	var networkDoc libvirtxml.Network
	if err := networkDoc.Unmarshal(networkXML); err != nil {
		return nil, "", err
	}

	for _, ip := range networkDoc.IPs {
		if ip.DHCP == nil {
			continue
		}
		for _, h := range ip.DHCP.Hosts {
			if _, exists := usedMACs[h.MAC]; !exists {
				return net.ParseIP(h.IP).To4(), h.MAC, nil
			}
		}
	}

	return nil, "", errors.Errorf("no free IPs in the network %q", n.Name)
}

func removeNetworkFromFirewall() error {
	c := &nftables.Conn{}
	chains, err := c.ListChains()
	if err != nil {
		return errors.WithStack(err)
	}

	for _, ch := range chains {
		rules, err := c.GetRules(ch.Table, ch)
		if err != nil {
			return errors.WithStack(err)
		}
		for _, r := range rules {
			for _, e := range r.Exprs {
				verdict, ok := e.(*expr.Verdict)
				if !ok || !strings.HasPrefix(verdict.Chain, "OSMAN_") {
					continue
				}
				if err := c.DelRule(r); err != nil {
					return errors.WithStack(err)
				}
				break
			}
		}
	}

	for _, ch := range chains {
		if strings.HasPrefix(ch.Name, "OSMAN_") {
			c.DelChain(ch)
		}
	}

	return errors.WithStack(c.Flush())
}

func deleteNetwork(l *libvirt.Libvirt, n libvirt.Network) error {
	if err := l.NetworkDestroy(n); err != nil {
		return errors.WithStack(err)
	}
	if err := l.NetworkUndefine(n); err != nil {
		return errors.WithStack(err)
	}

	return removeNetworkFromFirewall()
}

func forwardPorts(meta metadata, ip net.IP, buildID types.BuildID, n network) error {
	if len(meta.Forwards) == 0 {
		return nil
	}

	_, netIP, err := net.ParseCIDR(n.Network)
	if err != nil {
		return errors.WithStack(err)
	}

	c := &nftables.Conn{}

	chains, err := c.ListChains()
	if err != nil {
		return errors.WithStack(err)
	}

	var preroutingChain *nftables.Chain
	var postroutingChain *nftables.Chain
	var outputChain *nftables.Chain
	var osmanPreroutingChain *nftables.Chain
	var osmanPostroutingChain *nftables.Chain
	var osmanOutputChain *nftables.Chain
	for _, ch := range chains {
		if ch.Table == nil || ch.Table.Family != nftables.TableFamilyIPv4 {
			continue
		}
		switch ch.Name {
		case "PREROUTING":
			if ch.Type == nftables.ChainTypeNAT {
				preroutingChain = ch
			}
		case "POSTROUTING":
			if ch.Type == nftables.ChainTypeNAT {
				postroutingChain = ch
			}
		case "OUTPUT":
			if ch.Type == nftables.ChainTypeNAT {
				outputChain = ch
			}
		case "OSMAN_PREROUTING":
			osmanPreroutingChain = ch
		case "OSMAN_POSTROUTING":
			osmanPostroutingChain = ch
		case "OSMAN_OUTPUT":
			osmanOutputChain = ch
		}
	}

	if osmanPreroutingChain == nil {
		if preroutingChain == nil {
			var natTable *nftables.Table
			tables, err := c.ListTables()
			if err != nil {
				return errors.WithStack(err)
			}

			for _, t := range tables {
				if t.Family == nftables.TableFamilyIPv4 &&
					t.Name == tableNAT {
					natTable = t
					break
				}
			}

			if natTable == nil {
				return errors.New("no nat table")
			}

			preroutingChain = c.AddChain(&nftables.Chain{
				Name:     "PREROUTING",
				Table:    natTable,
				Type:     nftables.ChainTypeNAT,
				Hooknum:  nftables.ChainHookPrerouting,
				Priority: nftables.ChainPriorityNATDest,
			})
		}

		osmanPreroutingChain = c.AddChain(&nftables.Chain{
			Name:  "OSMAN_PREROUTING",
			Table: preroutingChain.Table,
		})
		c.AddRule(&nftables.Rule{
			Table: preroutingChain.Table,
			Chain: preroutingChain,
			Exprs: []expr.Any{
				&expr.Counter{},
				&expr.Verdict{
					Kind:  expr.VerdictJump,
					Chain: osmanPreroutingChain.Name,
				},
			},
		})
	}
	if osmanPostroutingChain == nil {
		if postroutingChain == nil {
			var natTable *nftables.Table
			tables, err := c.ListTables()
			if err != nil {
				return errors.WithStack(err)
			}

			for _, t := range tables {
				if t.Family == nftables.TableFamilyIPv4 &&
					t.Name == tableNAT {
					natTable = t
					break
				}
			}

			if natTable == nil {
				return errors.New("no nat table")
			}

			postroutingChain = c.AddChain(&nftables.Chain{
				Name:     "POSTROUTING",
				Table:    natTable,
				Type:     nftables.ChainTypeNAT,
				Hooknum:  nftables.ChainHookPostrouting,
				Priority: nftables.ChainPriorityNATSource,
			})
		}

		osmanPostroutingChain = c.AddChain(&nftables.Chain{
			Name:  "OSMAN_POSTROUTING",
			Table: postroutingChain.Table,
		})
		c.AddRule(&nftables.Rule{
			Table: postroutingChain.Table,
			Chain: postroutingChain,
			Exprs: []expr.Any{
				&expr.Counter{},
				&expr.Verdict{
					Kind:  expr.VerdictJump,
					Chain: osmanPostroutingChain.Name,
				},
			},
		})
	}
	if osmanOutputChain == nil {
		if outputChain == nil {
			var natTable *nftables.Table
			tables, err := c.ListTables()
			if err != nil {
				return errors.WithStack(err)
			}

			for _, t := range tables {
				if t.Family == nftables.TableFamilyIPv4 &&
					t.Name == tableNAT {
					natTable = t
					break
				}
			}

			if natTable == nil {
				return errors.New("no nat table")
			}

			outputChain = c.AddChain(&nftables.Chain{
				Name:     "OUTPUT",
				Table:    natTable,
				Type:     nftables.ChainTypeNAT,
				Hooknum:  nftables.ChainHookOutput,
				Priority: nftables.ChainPriorityNATSource,
			})
		}

		osmanOutputChain = c.AddChain(&nftables.Chain{
			Name:  "OSMAN_OUTPUT",
			Table: outputChain.Table,
		})
		c.AddRule(&nftables.Rule{
			Table: outputChain.Table,
			Chain: outputChain,
			Exprs: []expr.Any{
				&expr.Counter{},
				&expr.Verdict{
					Kind:  expr.VerdictJump,
					Chain: osmanOutputChain.Name,
				},
			},
		})
	}

	start := ip4ToUint32(netIP.IP) + 2
	end := start + netSize(netIP) - 4
	startIP := uint32ToIP4(start)
	endIP := uint32ToIP4(end)
	for _, f := range meta.Forwards {
		var proto byte
		switch f.Proto {
		case "tcp":
			proto = unix.IPPROTO_TCP
		case "udp":
			proto = unix.IPPROTO_UDP
		default:
			panic(errors.Errorf("unknown proto %q", f.Proto))
		}

		// forwarding traffic incoming requests
		c.AddRule(&nftables.Rule{
			Table:    osmanPreroutingChain.Table,
			Chain:    osmanPreroutingChain,
			UserData: []byte(buildID),
			Exprs: []expr.Any{
				&expr.Payload{
					DestRegister: 1,
					Base:         expr.PayloadBaseNetworkHeader,
					Offset:       16,
					Len:          4,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     f.PublicIP,
				},
				&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     []byte{proto},
				},
				&expr.Payload{
					DestRegister: 1,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     binaryutil.BigEndian.PutUint16(f.PublicPort),
				},
				&expr.Immediate{
					Register: 1,
					Data:     ip,
				},
				&expr.Immediate{
					Register: 2,
					Data:     binaryutil.BigEndian.PutUint16(f.VMPort),
				},
				&expr.Counter{},
				&expr.NAT{
					Type:        expr.NATTypeDestNAT,
					Family:      unix.NFPROTO_IPV4,
					RegAddrMin:  1,
					RegProtoMin: 2,
				},
			},
		})

		// forwarding traffic outgoing from the host machine
		c.AddRule(&nftables.Rule{
			Table:    osmanOutputChain.Table,
			Chain:    osmanOutputChain,
			UserData: []byte(buildID),
			Exprs: []expr.Any{
				&expr.Payload{
					DestRegister: 1,
					Base:         expr.PayloadBaseNetworkHeader,
					Offset:       16,
					Len:          4,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     f.PublicIP,
				},
				&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     []byte{proto},
				},
				&expr.Payload{
					DestRegister: 1,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     binaryutil.BigEndian.PutUint16(f.PublicPort),
				},
				&expr.Immediate{
					Register: 1,
					Data:     ip,
				},
				&expr.Immediate{
					Register: 2,
					Data:     binaryutil.BigEndian.PutUint16(f.VMPort),
				},
				&expr.Counter{},
				&expr.NAT{
					Type:        expr.NATTypeDestNAT,
					Family:      unix.NFPROTO_IPV4,
					RegAddrMin:  1,
					RegProtoMin: 2,
				},
			},
		})

		// forwarding traffic coming from the osman network (loop)
		c.AddRule(&nftables.Rule{
			Table:    osmanPostroutingChain.Table,
			Chain:    osmanPostroutingChain,
			UserData: []byte(buildID),
			Exprs: []expr.Any{
				&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     []byte(n.Name + "\x00"),
				},
				&expr.Payload{
					DestRegister: 1,
					Base:         expr.PayloadBaseNetworkHeader,
					Offset:       12,
					Len:          4,
				},
				&expr.Cmp{
					Op:       expr.CmpOpGte,
					Register: 1,
					Data:     startIP,
				},
				&expr.Cmp{
					Op:       expr.CmpOpLte,
					Register: 1,
					Data:     endIP,
				},
				&expr.Payload{
					DestRegister: 1,
					Base:         expr.PayloadBaseNetworkHeader,
					Offset:       16,
					Len:          4,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     ip,
				},
				&expr.Meta{Key: expr.MetaKeyL4PROTO, Register: 1},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     []byte{proto},
				},
				&expr.Payload{
					DestRegister: 1,
					Base:         expr.PayloadBaseTransportHeader,
					Offset:       2,
					Len:          2,
				},
				&expr.Cmp{
					Op:       expr.CmpOpEq,
					Register: 1,
					Data:     binaryutil.BigEndian.PutUint16(f.VMPort),
				},
				&expr.Counter{},
				&expr.Masq{},
			},
		})
	}

	return errors.WithStack(c.Flush())
}

func removeVMFirewallRules(buildID types.BuildID) error {
	c := &nftables.Conn{}
	chains, err := c.ListChains()
	if err != nil {
		return errors.WithStack(err)
	}

	for _, ch := range chains {
		if !strings.HasPrefix(ch.Name, "OSMAN_") {
			continue
		}

		rules, err := c.GetRules(ch.Table, ch)
		if err != nil {
			return errors.WithStack(err)
		}
		for _, r := range rules {
			if types.BuildID(r.UserData) != buildID {
				continue
			}
			if err := c.DelRule(r); err != nil {
				return errors.WithStack(err)
			}
		}
	}

	return errors.WithStack(c.Flush())
}

func removeVMFromNetwork(l *libvirt.Libvirt, n network, mac string, buildID types.BuildID) error {
	network, err := l.NetworkLookupByName(n.Name)
	if isError(err, libvirt.ErrNoNetwork) {
		return nil
	}
	if err != nil {
		return errors.WithStack(err)
	}

	domains, _, err := l.ConnectListAllDomains(1, libvirt.ConnectListDomainsActive|libvirt.ConnectListDomainsInactive)
	if err != nil {
		return errors.WithStack(err)
	}

	var networkNeeded bool
	for _, d := range domains {
		xml, err := l.DomainGetXMLDesc(d, 0)
		if err != nil {
			return errors.WithStack(err)
		}

		var domainDoc libvirtxml.Domain
		if err := domainDoc.Unmarshal(xml); err != nil {
			return errors.WithStack(err)
		}

		if domainDoc.Devices == nil {
			continue
		}

		for _, iface := range domainDoc.Devices.Interfaces {
			if iface.Source == nil || iface.Source.Network == nil || iface.Source.Network.Network != n.Name ||
				iface.MAC == nil || iface.MAC.Address == "" || iface.MAC.Address == mac {
				continue
			}
			networkNeeded = true
			break
		}
	}

	if networkNeeded {
		return removeVMFirewallRules(buildID)
	}
	return deleteNetwork(l, network)
}

type forward struct {
	PublicIP   net.IP
	PublicPort uint16
	VMPort     uint16
	Proto      string
}

func (f forward) String() string {
	return fmt.Sprintf("%s:%d:%d:%s", f.PublicIP, f.PublicPort, f.VMPort, f.Proto)
}

func (f forward) Key() string {
	return fmt.Sprintf("%s:%d:%s", f.PublicIP, f.PublicPort, f.Proto)
}

type metadata struct {
	BuildID  types.BuildID
	Forwards []forward
}

func parseMetadata(domainDoc libvirtxml.Domain) (metadata, error) {
	if domainDoc.Metadata == nil || domainDoc.Metadata.XML == "" {
		return metadata{}, nil
	}

	osmanDoc := etree.NewDocument()
	if err := osmanDoc.ReadFromString(domainDoc.Metadata.XML); err != nil {
		return metadata{}, errors.WithStack(err)
	}

	root := osmanDoc.Root()

	buildIDEl := root.FindElement("osman:buildID")
	if buildIDEl == nil || buildIDEl.Text() == "" {
		return metadata{}, errors.New("no build ID found in metadata")
	}

	meta := metadata{
		BuildID: types.BuildID(buildIDEl.Text()),
	}

	forwarded := map[string]struct{}{}
	for _, e := range root.FindElements("osman:forward") {
		rule := e.Text()
		parts1 := strings.SplitN(rule, ":", 3)
		if len(parts1) != 3 {
			return metadata{}, errors.Errorf("invalid forward rule %q", rule)
		}
		parts2 := strings.SplitN(parts1[2], "/", 2)
		if len(parts1) != 3 {
			return metadata{}, errors.Errorf("invalid forward rule %q", rule)
		}

		ipStr := parts1[0]
		hostPortStr := parts1[1]
		vmPortStr := parts2[0]
		proto := parts2[1]

		ip := net.ParseIP(ipStr)
		if ip == nil {
			return metadata{}, errors.Errorf("invalid forward rule %q", rule)
		}
		hostPort, err := strconv.Atoi(hostPortStr)
		if err != nil {
			return metadata{}, errors.Errorf("invalid forward rule %q", rule)
		}
		vmPort, err := strconv.Atoi(vmPortStr)
		if err != nil {
			return metadata{}, errors.Errorf("invalid forward rule %q", rule)
		}
		if proto != "tcp" && proto != "udp" {
			return metadata{}, errors.Errorf("invalid forward rule %q", rule)
		}

		f := forward{
			PublicIP:   ip.To4(),
			PublicPort: uint16(hostPort),
			VMPort:     uint16(vmPort),
			Proto:      proto,
		}

		dupKey := f.Key()
		if _, exists := forwarded[dupKey]; exists {
			return metadata{}, errors.Errorf("duplicated public endpoint in forward rule %q", rule)
		}

		forwarded[dupKey] = struct{}{}
		meta.Forwards = append(meta.Forwards, f)
	}

	return meta, nil
}

func prepareMetadata(domainDoc libvirtxml.Domain, info types.BuildInfo) (*libvirtxml.DomainMetadata, metadata, error) {
	if domainDoc.Metadata == nil {
		domainDoc.Metadata = &libvirtxml.DomainMetadata{}
	}
	osmanDoc := etree.NewDocument()
	if domainDoc.Metadata.XML == "" {
		root := etree.NewElement("osman:osman")
		root.CreateAttr("xmlns:osman", "http://go.exw.co/osman")
		osmanDoc.SetRoot(root)
	} else {
		if err := osmanDoc.ReadFromString(domainDoc.Metadata.XML); err != nil {
			return nil, metadata{}, errors.WithStack(err)
		}
	}
	root := osmanDoc.Root()
	if osmanDoc.Root().Tag != "osman" {
		return nil, metadata{}, errors.Errorf("osman:osman tag expected in metadata but %s found instead", root.Tag)
	}

	if root.FindElement("osman:buildID") != nil {
		return nil, metadata{}, errors.New("osman:buildID is a forbidden element in metadata")
	}

	buildID := root.CreateElement("osman:buildID")
	buildID.SetText(string(info.BuildID))

	metaLibvirtStr, err := osmanDoc.WriteToString()
	if err != nil {
		return nil, metadata{}, errors.WithStack(err)
	}
	domainDoc.Metadata.XML = metaLibvirtStr

	meta, err := parseMetadata(domainDoc)
	if err != nil {
		return nil, metadata{}, err
	}

	return domainDoc.Metadata, meta, nil
}

func prepareFilesystems(l *libvirt.Libvirt, domainDoc libvirtxml.Domain, dir string) ([]libvirtxml.DomainFilesystem, error) {
	items, err := os.ReadDir(dir)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		return nil, nil
	default:
		return nil, errors.WithStack(err)
	}

	if len(items) == 0 {
		return nil, nil
	}

	dirs := make(map[string]string, len(items))
	for _, i := range items {
		if !i.IsDir() {
			continue
		}
		dirs[filepath.Join(dir, i.Name())] = i.Name()
	}

	if len(dirs) == 0 {
		return nil, nil
	}

	domains, _, err := l.ConnectListAllDomains(1, libvirt.ConnectListDomainsActive|libvirt.ConnectListDomainsInactive)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	domainDocs := make([]libvirtxml.Domain, 0, len(domains))
	domainDocs = append(domainDocs, domainDoc)

	for _, d := range domains {
		xml, err := l.DomainGetXMLDesc(d, 0)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		var domainDoc libvirtxml.Domain
		if err := domainDoc.Unmarshal(xml); err != nil {
			return nil, errors.WithStack(err)
		}

		if domainDoc.Devices == nil || len(domainDoc.Devices.Filesystems) == 0 {
			continue
		}

		domainDocs = append(domainDocs, domainDoc)
	}

	for _, domainDoc := range domainDocs {
		for _, fs := range domainDoc.Devices.Filesystems {
			if fs.Source == nil || fs.Source.Mount == nil {
				continue
			}
			if _, exists := dirs[fs.Source.Mount.Dir]; exists {
				return nil, errors.Errorf("directory %s is already mounted in VM %s", fs.Source.Mount.Dir, domainDoc.Name)
			}
		}
	}

	filesystems := make([]libvirtxml.DomainFilesystem, 0, len(dirs))
	for dir, name := range dirs {
		filesystems = append(filesystems, libvirtxml.DomainFilesystem{
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
					Dir: dir,
				},
			},
			Target: &libvirtxml.DomainFilesystemTarget{
				Dir: name,
			},
		})
	}

	return filesystems, nil
}

func deployVM(
	ctx context.Context,
	l *libvirt.Libvirt,
	domainDoc libvirtxml.Domain,
	info types.BuildInfo,
	start config.Start,
) error {
	domainDoc.Name = start.MountKey.String()

	uuid, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	domainDoc.UUID = uuid.String()

	metaLibvirt, meta, err := prepareMetadata(domainDoc, info)
	if err != nil {
		return err
	}

	if len(meta.Forwards) > 0 {
		forwardedEndpoints, err := findForwardedEndpoints(l)
		if err != nil {
			return err
		}

		for _, f := range meta.Forwards {
			if name, exists := forwardedEndpoints[f.Key()]; exists {
				return errors.Errorf("forwarding rule %q colides with the one existing in %s", f, name)
			}
		}
	}

	ip, mac, err := findAvailableIPAndMAC(l, networkNAT)
	if err != nil {
		return err
	}

	domainDoc.Metadata = metaLibvirt
	if domainDoc.Devices == nil {
		domainDoc.Devices = &libvirtxml.DomainDeviceList{}
	}
	domainDoc.Devices.Interfaces = append(domainDoc.Devices.Interfaces,
		libvirtxml.DomainInterface{
			MAC: &libvirtxml.DomainInterfaceMAC{
				Address: mac,
			},
			Source: &libvirtxml.DomainInterfaceSource{
				Network: &libvirtxml.DomainInterfaceSourceNetwork{
					Network: networkNAT.Name,
				},
			},
			Model: &libvirtxml.DomainInterfaceModel{
				Type: "virtio",
			},
		},
	)

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

	filesystems, err := prepareFilesystems(l, domainDoc, filepath.Join(start.VolumeDir, start.MountKey.Name))
	if err != nil {
		return err
	}

	domainDoc.Devices.Filesystems = append(domainDoc.Devices.Filesystems, filesystems...)

	if domainDoc.OS == nil {
		domainDoc.OS = &libvirtxml.DomainOS{}
	}
	domainDoc.OS.Kernel = info.Mounted + "/boot/vmlinuz"
	domainDoc.OS.Initrd = info.Mounted + "/boot/initramfs.img"
	domainDoc.OS.Cmdline = strings.Join(append([]string{"root=virtiofs:root"}, info.Params...), " ")

	if domainDoc.CPU == nil || domainDoc.CPU.Topology == nil {
		return errors.New("cpu topology must be specified")
	}

	capabilitiesRaw, err := l.ConnectGetCapabilities()
	if err != nil {
		return errors.WithStack(err)
	}

	var capabilitiesDoc libvirtxml.Caps
	if err := capabilitiesDoc.Unmarshal(capabilitiesRaw); err != nil {
		return errors.WithStack(err)
	}

	if domainDoc.VCPU == nil || domainDoc.VCPU.Value == 0 {
		return errors.New("number of vcpus is not provided")
	}
	if domainDoc.CPU == nil {
		return errors.New("cpu settings are not provided")
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
		return errors.WithStack(err)
	}
	if len(availableVCPUs) < cores {
		return errors.New("vm requires more cores than available on host")
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

	domainXML, err := domainDoc.Marshal()
	if err != nil {
		return errors.WithStack(err)
	}
	domain, err := l.DomainDefineXML(domainXML)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := ensureNetwork(ctx, l, networkNAT); err != nil {
		return err
	}

	if err := l.DomainCreate(domain); err != nil {
		return errors.WithStack(err)
	}

	return forwardPorts(meta, ip, info.BuildID, networkNAT)
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
		buildIDTag := doc.FindElement("//metadata/osman:osman/osman:buildID")
		if buildIDTag == nil {
			continue
		}
		if buildID == types.BuildID(buildIDTag.Text()) {
			active, err := l.DomainIsActive(d)
			if err != nil {
				return errors.WithStack(err)
			}
			if active == 1 {
				return errors.Errorf("vm %q cannot be deleted because it is running", d.Name)
			}

			var domainDoc libvirtxml.Domain
			if err := domainDoc.Unmarshal(xml); err != nil {
				return errors.WithStack(err)
			}

			var macNAT string
			if domainDoc.Devices != nil {
				for _, i := range domainDoc.Devices.Interfaces {
					if i.Source == nil || i.Source.Network == nil {
						continue
					}
					if i.Source.Network.Network == networkNAT.Name && i.MAC != nil {
						macNAT = i.MAC.Address
					}
				}
			}

			if err := l.DomainUndefineFlags(d, libvirt.DomainUndefineManagedSave|libvirt.DomainUndefineSnapshotsMetadata|libvirt.DomainUndefineNvram|libvirt.DomainUndefineCheckpointsMetadata); err != nil && !libvirt.IsNotFound(err) {
				return errors.WithStack(err)
			}

			if macNAT != "" {
				return removeVMFromNetwork(l, networkNAT, macNAT, buildID)
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

func findForwardedEndpoints(l *libvirt.Libvirt) (map[string]string, error) {
	res := map[string]string{}

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

		meta, err := parseMetadata(domainDoc)
		if err != nil {
			return nil, err
		}

		for _, f := range meta.Forwards {
			res[f.Key()] = domainDoc.Name
		}
	}

	return res, nil
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

func defaultIface() (string, error) {
	routes, err := netlink.RouteList(nil, syscall.AF_UNSPEC)
	if err != nil {
		return "", errors.WithStack(err)
	}

	for _, r := range routes {
		if isDefaultRoute(r) {
			defaultIface, err := net.InterfaceByIndex(r.LinkIndex)
			if err != nil {
				return "", errors.WithStack(err)
			}
			return defaultIface.Name, nil
		}
	}
	return "", errors.New("default network interface not found")
}

func ipToMAC(ip net.IP) string {
	const template = "00:01:%02x:%02x:%02x:%02x"
	return fmt.Sprintf(template, ip[0], ip[1], ip[2], ip[3])
}

package auth

import (
	"net"
	"sort"
	"strings"
)

type networkInterface struct {
	name, kind string
	nets       []*net.IPNet
}
type homeNetworkState struct {
	nets     []*net.IPNet
	gateways []net.IP
	bridge   bool
	// virtual are the container/VM bridge and tunnel subnets: never home, never suggested.
	virtual []*net.IPNet
}

var automaticHome = discoverHomeNetworks()

func discoverHomeNetworks() homeNetworkState {
	interfaces, err := net.Interfaces()
	if err != nil {
		return homeNetworkState{}
	}
	kinds := interfaceKinds()
	var local []networkInterface
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		entry := networkInterface{name: iface.Name, kind: kinds[iface.Name]}
		for _, address := range addresses {
			if n, ok := address.(*net.IPNet); ok {
				entry.nets = append(entry.nets, n)
			}
		}
		local = append(local, entry)
	}
	return selectHomeNetworks(local, defaultGateways())
}
func selectHomeNetworks(interfaces []networkInterface, gateways []net.IP) homeNetworkState {
	result := homeNetworkState{gateways: gateways}
	for _, iface := range interfaces {
		name := strings.ToLower(iface.name)
		// Do not infer home from container/VM bridges or point-to-point/VPN tunnels. An admin can
		// explicitly add the real client subnet; a NAT gateway itself must never grant bypass.
		virtual := iface.kind == "veth" || iface.kind == "bridge" || iface.kind == "unknown-virtual" || iface.kind == "tun" || iface.kind == "wireguard" || strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "virbr") || strings.HasPrefix(name, "cni") || strings.HasPrefix(name, "tailscale") || strings.HasPrefix(name, "wg")
		if virtual {
			result.bridge = result.bridge || iface.kind == "veth"
			result.virtual = append(result.virtual, iface.nets...)
			continue
		}
		for _, n := range iface.nets {
			ones, bits := n.Mask.Size()
			if !n.IP.IsPrivate() || ones == 0 || bits == 0 {
				continue
			}
			result.nets = append(result.nets, &net.IPNet{IP: n.IP.Mask(n.Mask), Mask: n.Mask})
		}
	}
	return result
}

// HomeNetworkInfo describes automatic detection for Settings; no caller Host or DNS is consulted.
func HomeNetworkInfo() ([]string, bool) {
	nets := make([]string, 0, len(automaticHome.nets))
	for _, n := range automaticHome.nets {
		nets = append(nets, n.String())
	}
	sort.Strings(nets)
	return nets, automaticHome.bridge
}

// SuggestHomeNetwork is the /24 of the device viewing Settings, offered with one click when Arc
// can't find the home network itself (Docker bridge networking): a private IPv4 address that isn't
// home yet, isn't the NAT gateway (where every client would appear to come from) and isn't on a
// container or VM bridge (another container, such as an untrusted reverse proxy). An admin still
// chooses to add it; nothing becomes home automatically. "" when there is nothing to suggest.
func SuggestHomeNetwork(addr string) string {
	ip := net.ParseIP(addr).To4()
	if ip == nil || !ip.IsPrivate() || IsHomeIP(addr) {
		return ""
	}
	for _, gateway := range automaticHome.gateways {
		if gateway.Equal(ip) {
			return ""
		}
	}
	for _, n := range automaticHome.virtual {
		if n.Contains(ip) {
			return ""
		}
	}
	return (&net.IPNet{IP: ip.Mask(net.CIDRMask(24, 32)), Mask: net.CIDRMask(24, 32)}).String()
}

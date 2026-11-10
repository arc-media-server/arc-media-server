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

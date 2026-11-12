package auth

import (
	"net"
	"net/http/httptest"
	"testing"
)

func TestUnrelatedPrivatePeerDoesNotGrantHomeBypass(t *testing.T) {
	// Pick a private peer outside every server interface, as with an unrelated NAT/bridge peer.
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, peer := range []string{"172.31.255.254", "10.255.255.254", "192.168.255.254"} {
		ip := net.ParseIP(peer)
		onInterface := false
		for _, addr := range addrs {
			if n, ok := addr.(*net.IPNet); ok && n.Contains(ip) {
				onInterface = true
			}
		}
		if onInterface {
			continue
		}
		r := httptest.NewRequest("GET", "http://arc/api/auth/status", nil)
		r.RemoteAddr = peer + ":1234"
		if IsLocal(r) {
			t.Fatalf("private peer %s outside server interfaces granted home bypass", peer)
		}
		return
	}
	t.Fatal("no unrelated private test address")
}

func TestHomeNetworkDiscoveryExcludesBridgeButKeepsHostAndMacvlan(t *testing.T) {
	network := func(cidr string) *net.IPNet { ip, n, _ := net.ParseCIDR(cidr); n.IP = ip; return n }
	cases := []struct {
		name, kind       string
		wantHome, bridge bool
	}{
		{"eth0", "veth", false, true}, {"eth0", "unknown-virtual", false, false},
		{"docker0", "bridge", false, false}, {"eth0", "", true, false}, {"eth0", "macvlan", true, false},
	}
	saved := automaticHome
	defer func() { automaticHome = saved }()
	for _, tc := range cases {
		automaticHome = selectHomeNetworks([]networkInterface{{name: tc.name, kind: tc.kind, nets: []*net.IPNet{network("192.168.1.10/24")}}}, []net.IP{net.ParseIP("192.168.1.1")})
		if got := IsHomeIP("192.168.1.20"); got != tc.wantHome {
			t.Errorf("%s/%s peer home=%v", tc.name, tc.kind, got)
		}
		if IsHomeIP("192.168.1.1") {
			t.Errorf("%s/%s gateway grants home", tc.name, tc.kind)
		}
		if automaticHome.bridge != tc.bridge {
			t.Errorf("%s bridge warning=%v", tc.kind, automaticHome.bridge)
		}
	}
}

func TestSuggestHomeNetworkOnDockerBridge(t *testing.T) {
	saved := automaticHome
	defer func() { automaticHome = saved; SetHomeNetworks(nil) }()
	network := func(cidr string) *net.IPNet { ip, n, _ := net.ParseCIDR(cidr); n.IP = ip; return n }
	// Docker bridge: Arc's own interface is a veth on 172.17.0.0/16 behind gateway 172.17.0.1.
	automaticHome = selectHomeNetworks([]networkInterface{{name: "eth0", kind: "veth", nets: []*net.IPNet{network("172.17.0.2/16")}}}, []net.IP{net.ParseIP("172.17.0.1")})
	for addr, want := range map[string]string{
		"192.168.1.42": "192.168.1.0/24", // a laptop at home, seen through Docker's port mapping
		"10.0.5.9":     "10.0.5.0/24",
		"172.17.0.1":   "", // the gateway: every client would look like this through a userland proxy
		"172.17.0.5":   "", // another container on the bridge, such as a reverse proxy
		"81.2.69.160":  "", // public: away from home
		"100.100.1.1":  "", // Tailscale has its own button
		"127.0.0.1":    "",
		"fd00::1":      "",
		"not-an-ip":    "",
	} {
		if got := SuggestHomeNetwork(addr); got != want {
			t.Errorf("SuggestHomeNetwork(%q) = %q, want %q", addr, got, want)
		}
	}
	nets, _, err := ParseNetworks("192.168.1.0/24")
	if err != nil {
		t.Fatal(err)
	}
	SetHomeNetworks(nets)
	if got := SuggestHomeNetwork("192.168.1.42"); got != "" {
		t.Errorf("already home, still suggested %q", got)
	}
}

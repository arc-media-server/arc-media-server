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

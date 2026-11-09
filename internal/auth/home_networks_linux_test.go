//go:build linux

package auth

import "testing"

func TestLinuxGatewayParsingIncludesIPv4AndIPv6(t *testing.T) {
	v4 := []byte("eth0 00000000 0101A8C0 0003 0 0 0 00000000\neth0 0001A8C0 00000000 0001 0 0 0 00FFFFFF\n")
	v6 := []byte("00000000000000000000000000000000 00 00000000000000000000000000000000 00 fe800000000000000000000000000001 00000400 0 0 00000003 eth0\n")
	got := parseGateways(v4, v6)
	if len(got) != 2 || got[0].String() != "192.168.1.1" || got[1].String() != "fe80::1" {
		t.Fatalf("gateways=%v", got)
	}
}

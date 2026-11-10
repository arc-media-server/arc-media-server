package auth

import "testing"

func TestRejectAllAddressNetworkRanges(t *testing.T) {
	for _, raw := range []string{"0.0.0.0/0", "::/0", "192.168.1.0/0", "::ffff:0:0/0"} {
		if _, _, err := ParseNetworks(raw); err == nil {
			t.Errorf("universal network accepted: %s", raw)
		}
	}
	for _, raw := range []string{"192.168.1.0/24", "10.0.0.1", "fd00::/64"} {
		if _, _, err := ParseNetworks(raw); err != nil {
			t.Errorf("valid network refused: %s %v", raw, err)
		}
	}
}

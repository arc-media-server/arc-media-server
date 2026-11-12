package auth

import (
	"net/http/httptest"
	"testing"
)

func TestKnownServerHost(t *testing.T) {
	for _, tc := range []struct {
		host, domain string
		want         bool
	}{
		{"192.168.1.10:8099", "", true}, {"[::1]:8099", "", true}, {"localhost:8099", "", true},
		{"arc:8099", "", true}, {"arc.local:8099", "", true}, {"arc.home.arpa:8099", "", true},
		{"rebind.attacker.example:8099", "", false}, {"arc.local.evil.example", "", false},
		{"arc.example:8099", "arc.example", true}, {"evil.example", "arc.example", false},
	} {
		r := httptest.NewRequest("GET", "http://"+tc.host+"/", nil)
		if got := KnownServerHost(r, tc.domain); got != tc.want {
			t.Errorf("host %s: %v", tc.host, got)
		}
	}
	r := httptest.NewRequest("GET", "http://arc:8099/", nil)
	r.RemoteAddr = "192.168.1.2:8000"
	r.Header.Set("X-Forwarded-Host", "evil.example")
	if !KnownServerHost(r, "") {
		t.Fatal("untrusted forwarded host should not replace actual host")
	}
	nets, _, _ := ParseNetworks("192.168.1.2")
	SetTrustedProxies(nets)
	defer SetTrustedProxies(nil)
	if KnownServerHost(r, "") {
		t.Fatal("trusted proxy must check the browser's external host")
	}
	r.Header.Set("X-Forwarded-Host", "arc.example")
	if !KnownServerHost(r, "arc.example") {
		t.Fatal("configured external domain rejected")
	}
}

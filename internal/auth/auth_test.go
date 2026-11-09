package auth

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestForwardedRequestsAreNotLocal(t *testing.T) {
	for _, peer := range []string{"127.0.0.1:45000", "192.168.1.2:45000", "[::1]:45000"} {
		for header, value := range map[string]string{
			"Forwarded": "for=203.0.113.5;proto=https", "X-Forwarded-For": "203.0.113.5",
			"X-Real-IP": "127.0.0.1", "X-Forwarded-Host": "arc.example", "X-Forwarded-Proto": "https",
		} {
			r := httptest.NewRequest("GET", "http://arc/", nil)
			r.RemoteAddr = peer
			r.Header.Set(header, value)
			if IsLocal(r) {
				t.Errorf("forwarded request with %s via %s counted as local", header, peer)
			}
		}
	}
}

func TestIsLocal(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:5000", true},
		{"[::1]:5000", true},
		{"192.168.1.20:5000", true},
		{"10.0.0.5:5000", true},
		{"172.16.3.4:5000", true},
		{"[fe80::1]:5000", true},
		{"8.8.8.8:5000", false},
		{"[2001:4860::8888]:5000", false},
		{"garbage", false},
	}
	for _, tt := range tests {
		if got := IsLocal(&http.Request{RemoteAddr: tt.addr}); got != tt.want {
			t.Errorf("IsLocal(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestHashAndPIN(t *testing.T) {
	h, err := Hash("1234")
	if err != nil {
		t.Fatal(err)
	}
	if !Check(h, "1234") || Check(h, "4321") || Check("", "") {
		t.Error("Check gave the wrong answer")
	}
	for pin, ok := range map[string]bool{"1234": true, "0000": true, "123": false, "12345": false, "12a4": false} {
		if ValidPIN(pin) != ok {
			t.Errorf("ValidPIN(%q) = %v", pin, !ok)
		}
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter()
	for range maxFailures {
		if !l.Allow("a") {
			t.Fatal("blocked too early")
		}
		l.Fail("a")
	}
	if l.Allow("a") {
		t.Error("not blocked after too many failures")
	}
	if !l.Allow("b") {
		t.Error("other keys should be unaffected")
	}
	l.Succeed("a")
	if !l.Allow("a") {
		t.Error("success should clear failures")
	}
}

func TestTokens(t *testing.T) {
	a, ha := NewToken()
	b, _ := NewToken()
	if a == b || HashToken(a) != ha || len(a) < 40 {
		t.Error("tokens should be long, random and hash consistently")
	}
}

func TestHomeNetworks(t *testing.T) {
	nets, tidy, err := ParseNetworks("100.64.0.0/10, 203.0.113.5")
	if err != nil || tidy != "100.64.0.0/10, 203.0.113.5/32" {
		t.Fatalf("got %q %v", tidy, err)
	}
	if IsHomeIP("100.101.1.2") || !IsHomeIP("192.168.1.4") {
		t.Fatal("before setting networks")
	}
	SetHomeNetworks(nets)
	defer SetHomeNetworks(nil)
	if !IsHomeIP("100.101.1.2") || !IsHomeIP("203.0.113.5") || IsHomeIP("8.8.8.8") {
		t.Fatal("after setting networks")
	}
	if _, _, err := ParseNetworks("not a network"); err == nil {
		t.Fatal("bad input accepted")
	}
}

func init() {
	automaticHome = homeNetworkState{}
	for _, cidr := range []string{"192.168.1.0/24", "10.0.0.0/24", "172.16.3.0/24", "fe80::/64"} {
		_, n, _ := net.ParseCIDR(cidr)
		automaticHome.nets = append(automaticHome.nets, n)
	}
}

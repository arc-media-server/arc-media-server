package auth

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestTrustedProxies(t *testing.T) {
	proxies, _, err := ParseNetworks("172.17.0.5, 10.0.0.0/30")
	if err != nil {
		t.Fatal(err)
	}
	SetTrustedProxies(proxies)
	defer SetTrustedProxies(nil)

	req := func(peer string, headers ...string) *http.Request {
		r := &http.Request{RemoteAddr: peer, Header: http.Header{}}
		for i := 0; i+1 < len(headers); i += 2 {
			r.Header.Add(headers[i], headers[i+1])
		}
		return r
	}
	for _, tt := range []struct {
		name   string
		r      *http.Request
		local  bool
		client string
	}{
		{"trusted proxy relays a home client", req("172.17.0.5:4000", "X-Forwarded-For", "192.168.1.20"), true, "192.168.1.20"},
		{"trusted proxy relays someone on the internet", req("172.17.0.5:4000", "X-Forwarded-For", "203.0.113.9"), false, "203.0.113.9"},
		// The client can write its own X-Forwarded-For; only the hop the proxy added counts.
		{"spoofed left entry is ignored", req("172.17.0.5:4000", "X-Forwarded-For", "192.168.1.20, 203.0.113.9"), false, "203.0.113.9"},
		{"chain of trusted proxies is skipped", req("172.17.0.5:4000", "X-Forwarded-For", "203.0.113.9, 192.168.1.20, 10.0.0.2"), true, "192.168.1.20"},
		{"separate header lines form one chain", req("10.0.0.1:4000", "X-Forwarded-For", "203.0.113.9", "X-Forwarded-For", "192.168.1.30"), true, "192.168.1.30"},
		{"RFC 7239 Forwarded with IPv6 and port", req("172.17.0.5:4000", "Forwarded", `for="[2001:db8::1]:4711";proto=https`), false, "2001:db8::1"},
		{"Forwarded from home", req("172.17.0.5:4000", "Forwarded", "for=192.168.1.40;proto=http"), true, "192.168.1.40"},
		{"X-Real-IP alone", req("172.17.0.5:4000", "X-Real-IP", "192.168.1.50"), true, "192.168.1.50"},
		{"obfuscated client is not local", req("172.17.0.5:4000", "Forwarded", "for=unknown"), false, "172.17.0.5"},
		// Anyone else's headers grant nothing, exactly as before trusted proxies existed.
		{"untrusted proxy is never local", req("192.168.1.9:4000", "X-Forwarded-For", "192.168.1.20"), false, "192.168.1.9"},
		{"no headers: the connection itself", req("192.168.1.9:4000"), true, "192.168.1.9"},
		{"trusted proxy's own request without headers", req("172.17.0.5:4000"), true, "172.17.0.5"},
	} {
		if got := IsLocal(tt.r); got != tt.local {
			t.Errorf("%s: IsLocal = %v, want %v", tt.name, got, tt.local)
		}
		if got := ClientIP(tt.r); got != tt.client {
			t.Errorf("%s: ClientIP = %q, want %q", tt.name, got, tt.client)
		}
	}

	if !IsHTTPS(req("172.17.0.5:4000", "X-Forwarded-Proto", "https")) || !IsHTTPS(req("172.17.0.5:4000", "Forwarded", "for=203.0.113.9;proto=https")) {
		t.Error("a trusted proxy's HTTPS report was ignored")
	}
	if IsHTTPS(req("203.0.113.7:4000", "X-Forwarded-Proto", "https")) {
		t.Error("an untrusted client claimed HTTPS")
	}
	if r := req("203.0.113.7:4000"); IsHTTPS(&http.Request{RemoteAddr: r.RemoteAddr, Header: r.Header, TLS: &tls.ConnectionState{}}) != true {
		t.Error("a direct TLS connection isn't HTTPS")
	}

	// With no trusted proxies, forwarded requests are never local, as before.
	SetTrustedProxies(nil)
	if IsLocal(req("172.17.0.5:4000", "X-Forwarded-For", "192.168.1.20")) {
		t.Error("forwarded request counted as local without a trusted proxy")
	}
}

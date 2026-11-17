package remote

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNormalizePublicURL(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""}, {"  https://ARC.example.com:443/  ", "https://arc.example.com"},
		{"HTTP://localhost:8080/", "http://localhost:8080"}, {"https://[2001:db8::1]:8443", "https://[2001:db8::1]:8443"},
		{"http://arc.example.com:80", "http://arc.example.com"},
	} {
		got, err := NormalizePublicURL(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("%q: %q %v", tc.in, got, err)
		}
	}
	for _, bad := range []string{"arc.example.com", "//arc.example.com", "ftp://arc.example.com", "https://u:secret@arc.example.com", "https://arc.example.com/arc", "https://arc.example.com/%2f", "https://arc.example.com?", "https://arc.example.com?token=x", "https://arc.example.com#", "https://arc.example.com#x", "https://arc.example.com:0", "https://arc.example.com:65536", "https://arc.example.com:", "https://bad host", "https://bad..host", "https://-bad.example", "https://arc.example.com\nheader:x"} {
		if _, err := NormalizePublicURL(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	return New(NewCerts(t.TempDir()), NewMapper(slog.New(slog.NewTextHandler(io.Discard, nil))))
}

func TestPublicURLStatusSkipsAddressLookupAndKeepsMappingIndependent(t *testing.T) {
	var calls atomic.Int32
	ip := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); io.WriteString(w, "203.0.113.8") }))
	defer ip.Close()
	t.Setenv("AMS_PUBLIC_IP_URL", ip.URL)
	s := newTestService(t)
	s.Configure(true, false, 8099, 32400, "", "https://arc.example.com")
	got := s.Status(t.Context())
	if got.URL != "https://arc.example.com" || got.PublicIP != "" || got.DoubleNAT || calls.Load() != 0 || s.Mapper.enabled {
		t.Fatalf("override or mapping: %+v calls%d", got, calls.Load())
	}
	s.Configure(true, true, 8099, 32400, "", "https://arc.example.com")
	if !s.Mapper.enabled {
		t.Fatal("public URL must not silently override explicit router settings")
	}
	s.Configure(true, false, 8099, 32400, "", "")
	got = s.Status(t.Context())
	if got.URL != "http://203.0.113.8:32400" || calls.Load() != 1 {
		t.Fatal("automatic fallback", got)
	}
	s.Configure(false, false, 8099, 32400, "", "https://arc.example.com")
	if s.Status(t.Context()).URL != "" {
		t.Fatal("disabled remote access advertised a URL")
	}
}

func TestCustomPublicURLCheckUsesHealthAndDoesNotFollowRedirects(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalls.Add(1)
		io.WriteString(w, `{"name":"Arc Media Server"}`)
	}))
	defer target.Close()
	for _, mode := range []string{"ok", "wrong", "status", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/health" {
					t.Error("unexpected probe path", r.URL.Path)
				}
				switch mode {
				case "wrong":
					io.WriteString(w, `{"name":"Other"}`)
				case "status":
					w.WriteHeader(503)
					io.WriteString(w, `{"name":"Arc Media Server"}`)
				case "redirect":
					http.Redirect(w, r, target.URL, 302)
				default:
					io.WriteString(w, `{"name":"Arc Media Server"}`)
				}
			}))
			defer srv.Close()
			s := newTestService(t)
			s.Configure(true, false, 8099, 8099, "", srv.URL)
			err := s.Check(t.Context())
			if (err == nil) != (mode == "ok") {
				t.Fatalf("%s: %v", mode, err)
			}
			if err != nil && strings.Contains(err.Error(), srv.URL) {
				t.Fatal("check error exposed URL")
			}
		})
	}
	if targetCalls.Load() != 0 {
		t.Fatal("check followed redirect")
	}
}

func TestCustomPublicURLCheckRejectsUntrustedTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"name":"Arc Media Server"}`) }))
	defer srv.Close()
	s := newTestService(t)
	s.Configure(true, false, 8099, 8099, "", srv.URL)
	if s.Check(t.Context()) == nil {
		t.Fatal("custom origin bypassed TLS certificate verification")
	}
}

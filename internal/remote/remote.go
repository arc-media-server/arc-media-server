package remote

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Service ties remote access together: HTTPS, the router's port forwarding and the outside address.
type Service struct {
	Certs  *Certs
	Mapper *Mapper

	mu        sync.Mutex
	enabled   bool
	external  int // the port people outside connect to
	domain    string
	publicURL string
	ip        string
	ipAt      time.Time
}

func New(certs *Certs, mapper *Mapper) *Service { return &Service{Certs: certs, Mapper: mapper} }

// Configure applies the remote access settings.
func (s *Service) Configure(enabled, upnp bool, internal, external int, domain, publicURL string) {
	if external == 0 {
		external = internal
	}
	s.mu.Lock()
	s.enabled, s.external, s.domain, s.publicURL = enabled, external, domain, publicURL
	s.mu.Unlock()
	s.Mapper.Configure(enabled && upnp, internal, external)
}

// publicIPURL answers with the caller's address as plain text.
func publicIPURL() string {
	if u := os.Getenv("AMS_PUBLIC_IP_URL"); u != "" {
		return u
	}
	return "https://api.ipify.org"
}

// PublicIP is the home's outside address: from the router when it says, otherwise looked up
// (at most every 10 minutes, and only while remote access is on).
func (s *Service) PublicIP(ctx context.Context) string {
	if ip := s.Mapper.Status().ExternalIP; ip != "" && !isPrivate(ip) {
		return ip
	}
	s.mu.Lock()
	enabled, ip, at := s.enabled, s.ip, s.ipAt
	s.mu.Unlock()
	if !enabled {
		return ""
	}
	if time.Since(at) < 10*time.Minute {
		return ip
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ip = ""
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, publicIPURL(), nil); err == nil {
		if resp, err := http.DefaultClient.Do(req); err == nil {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
			resp.Body.Close()
			if parsed := net.ParseIP(strings.TrimSpace(string(b))); parsed != nil {
				ip = parsed.String()
			}
		}
	}
	s.mu.Lock()
	s.ip, s.ipAt = ip, time.Now()
	s.mu.Unlock()
	return ip
}

func isPrivate(ip string) bool {
	p := net.ParseIP(ip)
	// 100.64/10 is carrier-grade NAT: the router itself is behind another one.
	return p == nil || p.IsPrivate() || p.IsLoopback() || (p.To4() != nil && p.To4()[0] == 100 && p.To4()[1]&0xC0 == 64)
}

// Status is what Settings shows.
type Status struct {
	Enabled   bool `json:"enabled"`
	customURL bool // Snapshot: custom origins require normal TLS verification in Check.
	// The address to use away from home ("" until it's known).
	URL      string     `json:"url,omitempty"`
	PublicIP string     `json:"public_ip,omitempty"`
	Port     int        `json:"port"`
	UPnP     MapStatus  `json:"upnp"`
	HTTPS    CertStatus `json:"https"`
	// The router reports an address that isn't on the internet (it's behind another router, or
	// the provider shares addresses), so forwarding a port won't be enough.
	DoubleNAT bool `json:"double_nat,omitempty"`
}

func (s *Service) Status(ctx context.Context) Status {
	s.mu.Lock()
	enabled, port, domain, publicURL := s.enabled, s.external, s.domain, s.publicURL
	s.mu.Unlock()
	out := Status{Enabled: enabled, Port: port, UPnP: s.Mapper.Status(), HTTPS: s.Certs.Status(ctx)}
	if !enabled {
		return out
	}
	if publicURL != "" {
		out.URL, out.customURL = publicURL, true
		return out
	}
	out.DoubleNAT = out.UPnP.ExternalIP != "" && isPrivate(out.UPnP.ExternalIP)
	out.PublicIP = s.PublicIP(ctx)
	host := out.PublicIP
	if out.HTTPS.Mode == HTTPSLetsEncrypt && domain != "" {
		host = domain
	}
	if host != "" {
		scheme := "http"
		if out.HTTPS.Mode != HTTPSOff {
			scheme = "https"
		}
		out.URL = scheme + "://" + host
		if !(scheme == "https" && port == 443) && !(scheme == "http" && port == 80) {
			out.URL += ":" + strconv.Itoa(port)
		}
	}
	return out
}

// Check tries to reach Arc at its outside address. From inside the home that only works when the
// router allows it ("hairpinning"), so a failure isn't proof that it can't be reached.
func (s *Service) Check(ctx context.Context) error {
	st := s.Status(ctx)
	if !st.Enabled {
		return errors.New("remote access is off")
	}
	if st.URL == "" {
		return errors.New("your home's outside address couldn't be found")
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	client := &http.Client{Transport: &http.Transport{
		// Keep legacy certificate tolerance for inferred IPs; configured origins must verify TLS.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !st.customURL},
	}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, st.URL+"/api/health", nil)
	if err != nil {
		return errors.New("the outside address is invalid")
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("could not reach Arc at the outside address; check the address, routing and HTTPS certificate")
	}
	defer resp.Body.Close()
	var h struct {
		Name string `json:"name"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&h) != nil || h.Name != "Arc Media Server" {
		return errors.New("the outside address did not answer with Arc health; check the proxy destination")
	}
	return nil
}

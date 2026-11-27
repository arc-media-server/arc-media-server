package auth

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// KnownServerHost bounds operations that rely on the server's location rather than a
// password. Literal addresses cannot be DNS-rebound. Single-label intranet names, reserved
// local DNS suffixes, the machine's own hostname and explicitly configured names are allowed.
// Never resolve a caller-supplied name: a rebinding name would resolve to Arc as well.
// This does not classify the client as home; IsLocal must independently allow the client.
func KnownServerHost(r *http.Request, names ...string) bool {
	host := r.Host
	if isTrusted(peerIP(r)) {
		if f := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0]); f != "" {
			host = f
		}
	}
	u, err := url.Parse("http://" + host)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	name := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if name == "" {
		return false
	}
	if net.ParseIP(name) != nil {
		return true
	}
	machine, _ := os.Hostname()
	if strings.EqualFold(name, strings.TrimSuffix(machine, ".")) {
		return true
	}
	for _, configured := range names {
		if strings.EqualFold(name, strings.TrimSuffix(configured, ".")) {
			return true
		}
	}
	return !strings.Contains(name, ".") || strings.HasSuffix(name, ".local") || strings.HasSuffix(name, ".home.arpa")
}

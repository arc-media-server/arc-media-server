// Package auth holds the pieces of signing in: password and PIN hashing, session tokens, telling
// whether a request comes from the home network, and slowing down guessing.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLength is the shortest password accepted for the admin.
const MinPasswordLength = 8

var pinRe = regexp.MustCompile(`^\d{4}$`)

// ValidPIN reports whether pin is four digits.
func ValidPIN(pin string) bool { return pinRe.MatchString(pin) }

// Hash hashes a password or PIN for storing.
func Hash(secret string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	return string(b), err
}

// Check reports whether secret matches a hash from Hash. An empty hash never matches.
func Check(hash, secret string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}

// NewToken returns a random session token for the cookie, and the hash to store.
func NewToken() (token, hash string) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	token = base64.RawURLEncoding.EncodeToString(b[:])
	return token, HashToken(token)
}

// HashToken is what's stored for a session token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// IsLocal reports whether a request comes from this computer or the home network (a private
// address, or one in the extra home networks). Forwarded requests only qualify through a trusted
// proxy (SetTrustedProxies), judged by the client address it reports: any other proxy may be
// relaying someone on the internet, and header values from anyone else grant nothing.
func IsLocal(r *http.Request) bool {
	peer := peerIP(r)
	if !forwarded(r) {
		return IsHomeIP(peer)
	}
	if !isTrusted(peer) {
		return false
	}
	client := forwardedClient(r)
	return client != "" && IsHomeIP(client)
}

// ClientIP is the address a request comes from: the connection's, or for a trusted proxy the
// client address it reports. For rate limits, logs and the Dashboard, not for granting access.
func ClientIP(r *http.Request) string {
	peer := peerIP(r)
	if forwarded(r) && isTrusted(peer) {
		if client := forwardedClient(r); client != "" {
			return client
		}
	}
	return peer
}

// IsHTTPS reports whether the browser reached Arc over HTTPS: directly, or through a trusted proxy
// that says so (X-Forwarded-Proto or Forwarded proto=https).
func IsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if !isTrusted(peerIP(r)) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https") {
		return true
	}
	for _, e := range forwardedElements(r) {
		if strings.EqualFold(e["proto"], "https") {
			return true
		}
	}
	return false
}

var forwardHeaders = []string{"Forwarded", "X-Forwarded-For", "X-Real-IP", "X-Forwarded-Host", "X-Forwarded-Proto"}

func forwarded(r *http.Request) bool {
	for _, header := range forwardHeaders {
		if len(r.Header.Values(header)) > 0 {
			return true
		}
	}
	return false
}

func peerIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return strings.Trim(host, "[]")
}

// forwardedClient walks the forwarding chain from the nearest hop back, skipping trusted proxies:
// the first other address is the client ("" when it isn't a usable address). Entries further left
// were written by the client or untrusted hops and are ignored.
func forwardedClient(r *http.Request) string {
	var chain []string
	if els := forwardedElements(r); len(els) > 0 {
		for _, e := range els {
			chain = append(chain, e["for"])
		}
	} else if v := r.Header.Values("X-Forwarded-For"); len(v) > 0 {
		for _, line := range v {
			chain = append(chain, strings.Split(line, ",")...)
		}
	} else if v := r.Header.Get("X-Real-IP"); v != "" {
		chain = []string{v}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		ip := hostOnly(chain[i])
		if net.ParseIP(ip) == nil {
			return "" // "unknown" or an obfuscated name: no address to judge
		}
		if !isTrusted(ip) || i == 0 {
			return ip
		}
	}
	return ""
}

// forwardedElements parses RFC 7239 Forwarded headers into their elements' lower-case parameters.
func forwardedElements(r *http.Request) []map[string]string {
	var out []map[string]string
	for _, line := range r.Header.Values("Forwarded") {
		for _, element := range strings.Split(line, ",") {
			e := map[string]string{}
			for _, pair := range strings.Split(element, ";") {
				k, v, ok := strings.Cut(strings.TrimSpace(pair), "=")
				if ok {
					e[strings.ToLower(k)] = strings.Trim(strings.TrimSpace(v), `"`)
				}
			}
			out = append(out, e)
		}
	}
	return out
}

// hostOnly strips a port and IPv6 brackets: "[2001:db8::1]:4711" -> "2001:db8::1".
func hostOnly(s string) string {
	s = strings.TrimSpace(s)
	if host, _, err := net.SplitHostPort(s); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(s, "[]")
}

// trusted are reverse proxies whose forwarding headers Arc believes (Plex's and Jellyfin's
// "known proxies"). None by default.
var trusted struct {
	sync.RWMutex
	nets []*net.IPNet
}

// SetTrustedProxies sets the reverse proxies whose forwarding headers are believed.
func SetTrustedProxies(nets []*net.IPNet) {
	trusted.Lock()
	trusted.nets = nets
	trusted.Unlock()
}

func isTrusted(addr string) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	trusted.RLock()
	defer trusted.RUnlock()
	for _, n := range trusted.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// extraHome are networks that count as home besides private addresses (Plex's "LAN networks"),
// e.g. Tailscale's 100.64.0.0/10.
var extraHome struct {
	sync.RWMutex
	nets []*net.IPNet
}

// SetHomeNetworks sets the extra networks that count as home.
func SetHomeNetworks(nets []*net.IPNet) {
	extraHome.Lock()
	extraHome.nets = nets
	extraHome.Unlock()
}

// IsHomeIP reports whether an address is this computer or on the home network.
func IsHomeIP(addr string) bool {
	ip := net.ParseIP(addr)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return true
	}
	extraHome.RLock()
	defer extraHome.RUnlock()
	for _, n := range extraHome.nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ParseNetworks reads networks written one per line or separated by commas ("100.64.0.0/10", or a
// single address), returning them in a tidy form.
func ParseNetworks(s string) ([]*net.IPNet, string, error) {
	var nets []*net.IPNet
	var tidy []string
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == ' ' || r == ';' }) {
		if !strings.Contains(f, "/") {
			if ip := net.ParseIP(f); ip != nil {
				if ip.To4() != nil {
					f += "/32"
				} else {
					f += "/128"
				}
			}
		}
		_, n, err := net.ParseCIDR(f)
		if err != nil {
			return nil, "", errors.New(f + " isn't a network: write it like 100.64.0.0/10 or 203.0.113.5")
		}
		nets = append(nets, n)
		tidy = append(tidy, n.String())
	}
	return nets, strings.Join(tidy, ", "), nil
}

// ErrTooManyAttempts means too many wrong passwords or PINs were tried recently.
var ErrTooManyAttempts = errors.New("too many wrong attempts; wait a minute and try again")

// Limiter slows down guessing: after a few failures for a key (an address, or address + profile),
// further attempts are refused for a while.
type Limiter struct {
	mu       sync.Mutex
	failures map[string][]time.Time
}

const (
	maxFailures = 5
	window      = time.Minute
)

func NewLimiter() *Limiter { return &Limiter{failures: map[string][]time.Time{}} }

// Allow reports whether another attempt may be made for key.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.recent(key)) < maxFailures
}

// Fail records a wrong attempt.
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[key] = append(l.recent(key), time.Now())
}

// Succeed clears a key's failures.
func (l *Limiter) Succeed(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

func (l *Limiter) recent(key string) []time.Time {
	var keep []time.Time
	for _, t := range l.failures[key] {
		if time.Since(t) < window {
			keep = append(keep, t)
		}
	}
	if keep == nil {
		delete(l.failures, key)
	} else {
		l.failures[key] = keep
	}
	return keep
}

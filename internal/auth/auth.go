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
// address, or one in the extra home networks). Forwarding headers are ignored: they can be forged.
func IsLocal(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return IsHomeIP(strings.Trim(host, "[]"))
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

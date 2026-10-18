package remote

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// HTTPS modes.
const (
	HTTPSOff         = ""
	HTTPSFiles       = "files"       // your own certificate and key files
	HTTPSLetsEncrypt = "letsencrypt" // a free certificate for your domain, renewed automatically
)

// Certs provides the HTTPS certificate.
type Certs struct {
	cacheDir string

	mu       sync.Mutex
	mode     string
	certFile string
	keyFile  string
	domain   string
	le       *autocert.Manager
	file     *tls.Certificate
	fileMod  time.Time
	lastErr  string
	fetching bool
}

// NewCerts keeps Let's Encrypt certificates in cacheDir.
func NewCerts(cacheDir string) *Certs { return &Certs{cacheDir: cacheDir} }

var domainRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

// Check says what's wrong with HTTPS settings before they're used.
func Check(mode, certFile, keyFile, domain string) error {
	switch mode {
	case HTTPSOff:
		return nil
	case HTTPSFiles:
		if certFile == "" || keyFile == "" {
			return errors.New("choose the certificate file and its private key file")
		}
		_, err := loadPair(certFile, keyFile)
		return err
	case HTTPSLetsEncrypt:
		if !domainRe.MatchString(domain) {
			return errors.New("enter a domain name such as arc.example.com that points at your home's address")
		}
		return nil
	}
	return fmt.Errorf("unknown HTTPS mode %q", mode)
}

func loadPair(certFile, keyFile string) (*tls.Certificate, error) {
	c, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("can't use that certificate: %w", err)
	}
	if c.Leaf == nil && len(c.Certificate) > 0 {
		c.Leaf, _ = x509.ParseCertificate(c.Certificate[0])
	}
	return &c, nil
}

// Configure switches HTTPS to a mode. A Let's Encrypt certificate is asked for straight away.
func (c *Certs) Configure(mode, certFile, keyFile, domain string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if mode == c.mode && certFile == c.certFile && keyFile == c.keyFile && domain == c.domain {
		return
	}
	c.mode, c.certFile, c.keyFile, c.domain = mode, certFile, keyFile, domain
	c.le, c.file, c.lastErr = nil, nil, ""
	if mode == HTTPSLetsEncrypt {
		client := &acme.Client{DirectoryURL: autocert.DefaultACMEDirectory}
		if u := os.Getenv("AMS_ACME_URL"); u != "" {
			client.DirectoryURL = u // e.g. Let's Encrypt's staging server
		}
		c.le = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache(c.cacheDir),
			HostPolicy: autocert.HostWhitelist(domain),
			Client:     client,
		}
		go c.fetch()
	}
}

// fetch asks Let's Encrypt for the certificate now rather than on the first visit, so Settings can
// say whether it worked. Let's Encrypt checks the domain by connecting to it on port 443.
func (c *Certs) fetch() {
	c.mu.Lock()
	le, domain := c.le, c.domain
	if le == nil || c.fetching {
		c.mu.Unlock()
		return
	}
	c.fetching = true
	c.mu.Unlock()
	_, err := le.GetCertificate(helloFor(domain))
	c.mu.Lock()
	c.fetching = false
	if le == c.le {
		c.lastErr = ""
		if err != nil {
			c.lastErr = err.Error()
		}
	}
	c.mu.Unlock()
}

// helloFor is a TLS hello for a domain from a client that takes ECDSA certificates (smaller and
// faster than RSA), which is what autocert then asks for.
func helloFor(domain string) *tls.ClientHelloInfo {
	return &tls.ClientHelloInfo{
		ServerName:        domain,
		CipherSuites:      []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256},
		SignatureSchemes:  []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256},
		SupportedCurves:   []tls.CurveID{tls.CurveP256},
		SupportedVersions: []uint16{tls.VersionTLS13, tls.VersionTLS12},
	}
}

// Config is the TLS config for HTTPS connections, or nil while HTTPS is off.
func (c *Certs) Config() *tls.Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch c.mode {
	case HTTPSLetsEncrypt:
		if c.le == nil {
			return nil
		}
		cfg := c.le.TLSConfig()
		le := c.le
		cfg.GetCertificate = func(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if h.ServerName == "" {
				// Visits by IP address get the domain's certificate (with a name warning) rather
				// than a failed handshake.
				h.ServerName = c.domainName()
			}
			return le.GetCertificate(h)
		}
		cfg.NextProtos = []string{"http/1.1", acme.ALPNProto}
		return cfg
	case HTTPSFiles:
		return &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}, GetCertificate: c.fileCert}
	}
	return nil
}

func (c *Certs) domainName() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.domain
}

// fileCert loads the certificate files, again whenever they change (e.g. renewed by another tool).
func (c *Certs) fileCert(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	mod := modTime(c.certFile)
	if km := modTime(c.keyFile); km.After(mod) {
		mod = km
	}
	if c.file == nil || !mod.Equal(c.fileMod) {
		cert, err := loadPair(c.certFile, c.keyFile)
		if err != nil {
			c.lastErr = err.Error()
			if c.file == nil {
				return nil, err
			}
			return c.file, nil // keep using the last good one
		}
		c.file, c.fileMod, c.lastErr = cert, mod, ""
	}
	return c.file, nil
}

func modTime(path string) time.Time {
	if fi, err := os.Stat(path); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}

// Enabled reports whether HTTPS is on.
func (c *Certs) Enabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mode != HTTPSOff
}

// CertStatus is what Settings shows about the certificate.
type CertStatus struct {
	Mode string `json:"mode"`
	// The names it's for, and when it runs out (Unix seconds); 0 when there isn't one yet.
	Names   []string `json:"names,omitempty"`
	Expires int64    `json:"expires,omitempty"`
	// Asking Let's Encrypt for one now.
	Fetching bool   `json:"fetching,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Status describes the certificate in use.
func (c *Certs) Status(ctx context.Context) CertStatus {
	c.mu.Lock()
	mode, domain, fetching, lastErr := c.mode, c.domain, c.fetching, c.lastErr
	c.mu.Unlock()
	out := CertStatus{Mode: mode, Fetching: fetching, Error: lastErr}
	var leaf *x509.Certificate
	switch mode {
	case HTTPSFiles:
		if cert, err := c.fileCert(nil); err == nil {
			leaf = cert.Leaf
		} else {
			out.Error = err.Error()
		}
	case HTTPSLetsEncrypt:
		// autocert keeps the key and the chain in one PEM file named after the domain.
		if data, err := os.ReadFile(filepath.Join(c.cacheDir, domain)); err == nil {
			for rest := data; ; {
				var b *pem.Block
				if b, rest = pem.Decode(rest); b == nil {
					break
				}
				if b.Type == "CERTIFICATE" {
					leaf, _ = x509.ParseCertificate(b.Bytes)
					break
				}
			}
		}
	}
	if leaf != nil {
		out.Names, out.Expires = leaf.DNSNames, leaf.NotAfter.Unix()
		if len(out.Names) == 0 {
			out.Names = []string{leaf.Subject.CommonName}
		}
	}
	if out.Error != "" {
		out.Error = strings.TrimPrefix(out.Error, "acme/autocert: ")
	}
	return out
}

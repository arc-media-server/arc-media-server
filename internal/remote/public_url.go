package remote

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// NormalizePublicURL accepts an origin, not a path-prefixed deployment or a credential URL.
// Saving it never resolves the host or makes a network request.
func NormalizePublicURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	invalid := errors.New("public server URL must be an absolute http:// or https:// address, without credentials, a path, query or fragment")
	if len(value) > 2048 || strings.ContainsAny(value, "\\\r\n\t ") {
		return "", invalid
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", invalid
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.Opaque != "" || (u.EscapedPath() != "" && u.EscapedPath() != "/") || u.RawQuery != "" || u.ForceQuery || strings.Contains(value, "#") {
		return "", invalid
	}
	host := strings.ToLower(u.Hostname())
	if net.ParseIP(host) == nil {
		if len(strings.TrimSuffix(host, ".")) > 253 {
			return "", invalid
		}
		for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return "", invalid
			}
			for _, c := range label {
				if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
					return "", invalid
				}
			}
		}
	}
	port := u.Port()
	if strings.HasSuffix(u.Host, ":") {
		return "", invalid
	}
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", invalid
		}
		port = strconv.Itoa(n)
	}
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		u.Host = "[" + host + "]"
	} else {
		u.Host = host
	}
	u.Path, u.RawPath = "", ""
	return u.String(), nil
}

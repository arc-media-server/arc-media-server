package metadata

import "net/http"

// pooled is an HTTP transport that keeps enough connections open for a scan's parallel lookups
// (Go's default keeps only two per host, so the rest would each start a new TLS connection).
func pooled() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConnsPerHost = 16
	return t
}

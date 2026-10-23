package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"
)

type cacheMode struct{ read, write bool }
type cacheModeKey struct{}

// WithPreviewCache retains successful provider responses for the subsequent import.
func WithPreviewCache(ctx context.Context) context.Context {
	return context.WithValue(ctx, cacheModeKey{}, cacheMode{read: true, write: true})
}

// WithPreviewReuse reuses preview work without making ordinary refreshes stale.
func WithPreviewReuse(ctx context.Context) context.Context {
	return context.WithValue(ctx, cacheModeKey{}, cacheMode{read: true})
}

// WithFresh bypasses preview responses when new files require newer provider data.
func WithFresh(ctx context.Context) context.Context {
	return context.WithValue(ctx, cacheModeKey{}, cacheMode{})
}

type cachedResponse struct {
	data []byte
	at   time.Time
}

// Per-client, bounded, short-lived cache. The full request (including key and language)
// identifies an entry; failed responses are never cached. No user tuning is needed.
type responseCache struct {
	mu      sync.Mutex
	entries map[string]cachedResponse
	bytes   int
}

func (c *responseCache) load(ctx context.Context, key string, out any) bool {
	mode, _ := ctx.Value(cacheModeKey{}).(cacheMode)
	if !mode.read {
		c.mu.Lock()
		if old, ok := c.entries[key]; ok {
			delete(c.entries, key)
			c.bytes -= len(old.data)
		}
		c.mu.Unlock()
		return false
	}
	c.mu.Lock()
	e, ok := c.entries[key]
	if ok && time.Since(e.at) > 2*time.Hour {
		delete(c.entries, key)
		c.bytes -= len(e.data)
		ok = false
	}
	c.mu.Unlock()
	return ok && json.Unmarshal(e.data, out) == nil
}

func (c *responseCache) decode(ctx context.Context, key string, body io.Reader, out any) error {
	mode, _ := ctx.Value(cacheModeKey{}).(cacheMode)
	if !mode.write {
		return json.NewDecoder(body).Decode(out)
	}
	var captured cappedResponse
	if err := json.NewDecoder(io.TeeReader(body, &captured)).Decode(out); err != nil {
		return err
	}
	if captured.large {
		return nil
	}
	data := captured.Bytes()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil || c.bytes+len(data) > 64<<20 || len(c.entries) >= 4096 {
		c.entries = map[string]cachedResponse{}
		c.bytes = 0
	}
	if old, ok := c.entries[key]; ok {
		c.bytes -= len(old.data)
	}
	c.entries[key] = cachedResponse{data: bytes.Clone(data), at: time.Now()}
	c.bytes += len(data)
	return nil
}

type cappedResponse struct {
	bytes.Buffer
	large bool
}

func (b *cappedResponse) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 2<<20 {
		b.large = true
		return len(p), nil
	}
	if !b.large {
		_, _ = b.Buffer.Write(p)
	}
	return len(p), nil
}

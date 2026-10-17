// Package artwork keeps a local copy of provider artwork (posters, backdrops, episode stills) so
// images load fast, keep working offline and aren't fetched from TMDB/TVDB on every view.
// Files live in <data dir>/artwork, named by a hash of their source URL. They are not media files:
// the cache can be deleted at any time and is rebuilt as images are needed.
package artwork

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ams/internal/metadata"
)

// Images larger than this are refused; real posters and backdrops are a few MB at most.
const maxImageBytes = 25 << 20

// Source returns the provider URL for an item's image, picking a size that suits how it's shown.
// imageType is "primary" (poster, or still for episodes) or "backdrop".
func Source(itemKind, imageType, imagePath string) string {
	if imagePath == "" {
		return ""
	}
	size := "w500"
	switch {
	case imageType == "backdrop":
		size = "w1280"
	case itemKind == "episode":
		size = "w780" // episode stills are landscape
	case itemKind == "person":
		size = "w185" // small round photos
	}
	return metadata.ImageURL(imagePath, size)
}

// Key identifies a cached image by its source URL. It is also used as a version in image links,
// so a changed image gets a new link and browsers can cache each one forever.
func Key(sourceURL string) string {
	sum := sha256.Sum256([]byte(sourceURL))
	return hex.EncodeToString(sum[:12])
}

type Cache struct {
	dir  string
	http *http.Client
	log  *slog.Logger

	mu       sync.Mutex
	inflight map[string]*fetch
}

type fetch struct {
	done chan struct{}
	err  error
}

func New(dir string, log *slog.Logger) *Cache {
	return &Cache{
		dir:      dir,
		http:     &http.Client{Timeout: 30 * time.Second},
		log:      log,
		inflight: map[string]*fetch{},
	}
}

func (c *Cache) file(key string) string { return filepath.Join(c.dir, key[:2], key) }

// Has reports whether the image is already cached.
func (c *Cache) Has(sourceURL string) bool {
	_, err := os.Stat(c.file(Key(sourceURL)))
	return err == nil
}

// Open returns the cached copy of an image, downloading it first if needed. Concurrent requests
// for the same image share one download.
func (c *Cache) Open(ctx context.Context, sourceURL string) (*os.File, error) {
	key := Key(sourceURL)
	if f, err := os.Open(c.file(key)); err == nil {
		return f, nil
	}
	if err := c.download(ctx, sourceURL, key); err != nil {
		return nil, err
	}
	return os.Open(c.file(key))
}

func (c *Cache) download(ctx context.Context, sourceURL, key string) error {
	c.mu.Lock()
	if f := c.inflight[key]; f != nil {
		c.mu.Unlock()
		select {
		case <-f.done:
			return f.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f := &fetch{done: make(chan struct{})}
	c.inflight[key] = f
	c.mu.Unlock()

	// Not tied to one request: a viewer navigating away shouldn't cancel a download others wait on.
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
	f.err = c.fetchToDisk(dctx, sourceURL, key)
	cancel()

	c.mu.Lock()
	delete(c.inflight, key)
	c.mu.Unlock()
	close(f.done)
	return f.err
}

func (c *Cache) fetchToDisk(ctx context.Context, sourceURL, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("artwork %s: %s", sourceURL, resp.Status)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "image/") {
		return fmt.Errorf("artwork %s: not an image (%s)", sourceURL, ct)
	}

	dst := c.file(key)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), key+".*.tmp")
	if err != nil {
		return err
	}
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, maxImageBytes+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxImageBytes {
		err = fmt.Errorf("artwork %s: larger than %d MB", sourceURL, maxImageBytes>>20)
	}
	if err == nil && n == 0 {
		err = fmt.Errorf("artwork %s: empty response", sourceURL)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// Prefetch downloads any of the given images that aren't cached yet, a few at a time, so
// artwork is available offline. It stops early when ctx is cancelled.
func (c *Cache) Prefetch(ctx context.Context, sourceURLs []string) (downloaded, failed int) {
	const workers = 4
	todo := make(chan string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range todo {
				err := c.download(ctx, u, Key(u))
				mu.Lock()
				if err != nil {
					failed++
					if failed <= 3 {
						c.log.Debug("artwork download failed", "url", u, "err", err)
					}
				} else {
					downloaded++
				}
				mu.Unlock()
			}
		}()
	}
	for _, u := range sourceURLs {
		if ctx.Err() != nil {
			break
		}
		if u == "" || c.Has(u) {
			continue
		}
		todo <- u
	}
	close(todo)
	wg.Wait()
	return downloaded, failed
}

// Prune deletes cached images whose source is no longer used by any item, and leftover
// partial downloads. It returns how many files were removed.
func (c *Cache) Prune(inUse []string) (int, error) {
	keep := make(map[string]bool, len(inUse))
	for _, u := range inUse {
		if u != "" {
			keep[Key(u)] = true
		}
	}
	removed := 0
	err := filepath.WalkDir(c.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		stale := strings.HasSuffix(name, ".tmp")
		if stale {
			// A download may be writing it right now; only clear ones that were abandoned.
			if info, err := d.Info(); err == nil && time.Since(info.ModTime()) < time.Hour {
				stale = false
			}
		} else {
			stale = !keep[name]
		}
		if stale {
			if err := os.Remove(p); err == nil {
				removed++
			}
		}
		return nil
	})
	return removed, err
}

// Keeper keeps the cache in step with the library in the background: after each Trigger it
// downloads images that aren't cached yet and removes ones no longer used. Triggers that arrive
// while it's busy are merged into one more pass.
type Keeper struct {
	cache *Cache
	list  func(ctx context.Context) ([]string, error)
	wake  chan struct{}
}

// Keep starts a Keeper. list returns every image source currently in use, most important first.
func (c *Cache) Keep(ctx context.Context, list func(ctx context.Context) ([]string, error)) *Keeper {
	k := &Keeper{cache: c, list: list, wake: make(chan struct{}, 1)}
	go k.loop(ctx)
	return k
}

func (k *Keeper) Trigger() {
	select {
	case k.wake <- struct{}{}:
	default: // a pass is already queued
	}
}

func (k *Keeper) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-k.wake:
		}
		sources, err := k.list(ctx)
		if err != nil {
			if ctx.Err() == nil {
				k.cache.log.Warn("artwork: listing images failed", "err", err)
			}
			continue
		}
		downloaded, failed := k.cache.Prefetch(ctx, sources)
		if ctx.Err() != nil {
			return
		}
		// Only prune after a complete pass, so a half-finished list never deletes anything in use.
		removed, err := k.cache.Prune(sources)
		if err != nil {
			k.cache.log.Warn("artwork: cleanup failed", "err", err)
		}
		if downloaded+failed+removed > 0 {
			k.cache.log.Info("artwork cache updated", "downloaded", downloaded, "failed", failed, "removed", removed)
		}
	}
}

// ContentName returns a file name whose extension tells http.ServeContent the image type. With
// no recognised extension the type is detected from the file's contents.
func ContentName(sourceURL string) string {
	ext := strings.ToLower(path.Ext(sourceURL))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return "image" + ext
	}
	return "image"
}

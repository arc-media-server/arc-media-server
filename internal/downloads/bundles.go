package downloads

import (
	"ams/internal/artwork"
	"ams/internal/metadata"
	"ams/internal/store"
	"ams/internal/subtitles"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type BundleItem struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Title       string         `json:"title"`
	Overview    string         `json:"overview,omitempty"`
	Year        int            `json:"year,omitempty"`
	SeriesID    string         `json:"series_id,omitempty"`
	SeriesTitle string         `json:"series_title,omitempty"`
	Season      *int           `json:"season,omitempty"`
	Episode     *int           `json:"episode,omitempty"`
	DurationMS  int64          `json:"duration_ms"`
	Container   string         `json:"container,omitempty"`
	Streams     []store.Stream `json:"streams"`
}
type BundleAsset struct {
	ID              string `json:"id"`
	Role            string `json:"role"`
	MediaType       string `json:"media_type"`
	Size            int64  `json:"size"`
	SHA256          string `json:"sha256"`
	Language        string `json:"language,omitempty"`
	Forced          bool   `json:"forced,omitempty"`
	HearingImpaired bool   `json:"hearing_impaired,omitempty"`
	MTimeNS         int64  `json:"mtime_ns"`
}
type MissingAsset struct {
	Role     string `json:"role"`
	Reason   string `json:"reason_code"`
	Language string `json:"language,omitempty"`
}
type Bundle struct {
	ServerID    string         `json:"server_id"`
	Item        BundleItem     `json:"item"`
	Assets      []BundleAsset  `json:"assets"`
	Unavailable []MissingAsset `json:"unavailable"`
}
type bundleControl struct {
	mu           sync.Mutex
	dir, uploads string
	artwork      *artwork.Cache
	subtitles    *subtitles.Cache
	readers      map[string]int
	building     map[string]bool
}

// ConfigureBundles is called before workers/requests start. Only Arc-owned derived assets use dir.
func (m *Manager) ConfigureBundles(dir, uploads string, art *artwork.Cache, subs *subtitles.Cache) {
	m.bundles.dir, m.bundles.uploads, m.bundles.artwork, m.bundles.subtitles = dir, uploads, art, subs
}
func (m *Manager) BundlesAvailable() bool { return m.bundles.dir != "" }

func safeComponent(s string) bool {
	if len(s) < 16 || len(s) > 80 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
func (m *Manager) bundlePath(d *store.Download) string {
	return filepath.Join(m.bundles.dir, d.ID, d.Revision)
}

func (m *Manager) buildBundle(ctx context.Context, d *store.Download) (string, func(), error) {
	it, err := m.store.GetItem(ctx, d.ItemID)
	if err != nil || it == nil {
		return "", nil, errors.New("download metadata unavailable")
	}
	files, err := m.store.ItemFiles(ctx, d.ItemID)
	if err != nil {
		return "", nil, err
	}
	var file *store.MediaFile
	for i := range files {
		if files[i].ID == d.FileID {
			file = &files[i]
		}
	}
	if file == nil || file.CacheKey != d.SourceCacheKey {
		return "", nil, errors.New("download source changed")
	}
	server, err := m.store.Setting(ctx, "server_id")
	if err != nil {
		return "", nil, err
	}
	b := Bundle{ServerID: server, Item: BundleItem{ID: it.ID, Kind: it.Kind, Title: it.Title, Overview: it.Overview, Year: it.Year, DurationMS: file.DurationMS, Container: file.Container, Streams: file.Streams}, Assets: []BundleAsset{}, Unavailable: []MissingAsset{}}
	artItem := it
	if it.Kind == "episode" {
		parent, e := m.store.GetItem(ctx, it.ParentID)
		if e != nil {
			return "", nil, e
		}
		b.Item.SeriesID = it.ParentID
		if parent != nil {
			b.Item.SeriesTitle = parent.Title
			artItem = parent
			lib, e := m.store.GetLibrary(ctx, parent.LibraryID)
			if e != nil {
				return "", nil, e
			}
			if lib != nil {
				ordering, e := m.store.DisplayOrdering(ctx, parent.ID, lib.SourceOrdering)
				if e != nil {
					return "", nil, e
				}
				if ordering != 0 {
					entries, e := m.store.DisplayEntries(ctx, ordering)
					if e != nil {
						return "", nil, e
					}
					for _, entry := range entries {
						if entry.EpisodeID == it.ID {
							s, n := entry.Season, entry.Episode
							b.Item.Season, b.Item.Episode = &s, &n
							break
						}
					}
				}
			}
		}
	}
	var stage, final string
	c := &m.bundles
	if c.dir != "" {
		if !safeComponent(d.ID) || !safeComponent(d.Revision) {
			return "", nil, errors.New("invalid bundle identity")
		}
		c.mu.Lock()
		if c.building == nil {
			c.building = map[string]bool{}
		}
		if c.building[d.ID] {
			c.mu.Unlock()
			return "", nil, errors.New("bundle already preparing")
		}
		c.building[d.ID] = true
		c.mu.Unlock()
		defer func() { c.mu.Lock(); delete(c.building, d.ID); c.mu.Unlock() }()
		if err = os.MkdirAll(c.dir, 0700); err != nil {
			return "", nil, errors.New("bundle storage unavailable")
		}
		stage, err = os.MkdirTemp(c.dir, ".stage-"+d.ID+"-")
		if err != nil {
			return "", nil, errors.New("bundle storage unavailable")
		}
		defer os.RemoveAll(stage)
		final = m.bundlePath(d)
	}
	var total int64
	copyAsset := func(role, path string, cached *os.File, typ, lang string, forced, sdh bool) error {
		if stage == "" {
			if cached != nil {
				cached.Close()
			}
			b.Unavailable = append(b.Unavailable, MissingAsset{role, "storage_unavailable", lang})
			return nil
		}
		var f *os.File
		var e error
		if cached != nil {
			f = cached
		} else {
			f, e = os.Open(path)
		}
		if e != nil {
			b.Unavailable = append(b.Unavailable, MissingAsset{role, "not_available", lang})
			return nil
		}
		defer f.Close()
		info, e := f.Stat()
		limit := int64(5 << 20)
		if role == "subtitle" {
			limit = 2 << 20
		}
		if e != nil || !info.Mode().IsRegular() || info.Size() > limit || total+info.Size() > 24<<20 {
			b.Unavailable = append(b.Unavailable, MissingAsset{role, "size_or_type_unsupported", lang})
			return nil
		}
		data, e := io.ReadAll(io.LimitReader(f, limit+1))
		if e != nil || int64(len(data)) > limit {
			return errors.New("bundle asset read failed")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		after, e := f.Stat()
		if e != nil || !same(info, after) {
			return errors.New("bundle asset changed")
		}
		if role == "subtitle" && !utf8.Valid(data) {
			b.Unavailable = append(b.Unavailable, MissingAsset{role, "encoding_unsupported", lang})
			return nil
		}
		if typ == "" {
			typ = http.DetectContentType(data)
			if !strings.HasPrefix(typ, "image/") {
				b.Unavailable = append(b.Unavailable, MissingAsset{role, "format_unsupported", lang})
				return nil
			}
		}
		asset := BundleAsset{ID: store.NewID(), Role: role, MediaType: typ, Size: int64(len(data)), Language: lang, Forced: forced, HearingImpaired: sdh}
		sum := sha256.Sum256(data)
		asset.SHA256 = hex.EncodeToString(sum[:])
		dest := filepath.Join(stage, asset.ID)
		if e = os.WriteFile(dest, data, 0600); e != nil {
			return errors.New("bundle storage unavailable")
		}
		saved, e := os.Stat(dest)
		if e != nil {
			return errors.New("bundle storage unavailable")
		}
		asset.MTimeNS = saved.ModTime().UnixNano()
		total += asset.Size
		b.Assets = append(b.Assets, asset)
		return nil
	}
	for _, image := range []struct{ role, source, local string }{{"poster", artItem.PosterPath, artItem.LocalPoster}, {"backdrop", artItem.BackdropPath, artItem.LocalBackdrop}, {"logo", artItem.LogoPath, artItem.LocalLogo}} {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		local := image.local
		if name, ok := strings.CutPrefix(image.source, metadata.UploadPrefix); ok {
			local = filepath.Join(c.uploads, filepath.Base(name))
		}
		if local != "" {
			err = copyAsset(image.role, local, nil, "", "", false, false)
		} else if image.source != "" && c.artwork != nil {
			typ := image.role
			if typ == "poster" {
				typ = "primary"
			}
			cached, e := c.artwork.OpenCached(artwork.Source(artItem.Kind, typ, image.source))
			if e == nil {
				err = copyAsset(image.role, "", cached, "", "", false, false)
			} else {
				b.Unavailable = append(b.Unavailable, MissingAsset{image.role, "not_cached", ""})
			}
		} else {
			b.Unavailable = append(b.Unavailable, MissingAsset{image.role, "not_available", ""})
		}
		if err != nil {
			return "", nil, err
		}
	}
	tracks := subtitles.Tracks(file)
	if c.subtitles != nil {
		tracks = c.subtitles.Tracks(file)
	}
	external := 0
	for _, track := range tracks {
		if !track.External {
			continue
		}
		external++
		typ := ""
		switch track.Codec {
		case "subrip", "srt":
			typ = "application/x-subrip"
		case "webvtt":
			typ = "text/vtt"
		}
		if typ == "" || external > 8 {
			b.Unavailable = append(b.Unavailable, MissingAsset{"subtitle", "format_or_count_unsupported", track.Language})
			continue
		}
		if err = copyAsset("subtitle", track.Path, nil, typ, track.Language, track.Forced, track.HearingImpaired); err != nil {
			return "", nil, err
		}
	}
	encoded, err := json.Marshal(b)
	if err != nil || len(encoded) > 256<<10 {
		return "", nil, errors.New("bundle metadata too large")
	}
	if stage == "" {
		return string(encoded), nil, nil
	}
	// Bound Arc-owned derived storage, not original media. Include staging in the budget.
	c.mu.Lock()
	defer c.mu.Unlock()
	var used int64
	err = filepath.WalkDir(c.dir, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.Type().IsRegular() {
			info, e := entry.Info()
			if e != nil {
				return e
			}
			used += info.Size()
		}
		if used > 256<<20 {
			return errors.New("bundle storage budget reached")
		}
		return nil
	})
	if err != nil {
		return "", nil, errors.New("bundle storage budget unavailable")
	}
	if _, e := os.Stat(final); !os.IsNotExist(e) {
		return "", nil, errors.New("bundle revision already exists")
	}
	if err = os.MkdirAll(filepath.Dir(final), 0700); err != nil {
		return "", nil, errors.New("bundle storage unavailable")
	}
	if err = os.Rename(stage, final); err != nil {
		return "", nil, errors.New("bundle publication failed")
	}
	return string(encoded), func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.readers[d.ID] == 0 {
			_ = os.RemoveAll(final)
		}
	}, nil
}

// LeaseBundle protects published immutable files from cleanup until a reader returns.
func (m *Manager) LeaseBundle(d *store.Download) func() {
	c := &m.bundles
	c.mu.Lock()
	if c.readers == nil {
		c.readers = map[string]int{}
	}
	c.readers[d.ID]++
	c.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			c.readers[d.ID]--
			if c.readers[d.ID] == 0 {
				delete(c.readers, d.ID)
			}
			c.mu.Unlock()
		})
	}
}
func (m *Manager) OpenBundleAsset(d *store.Download, asset BundleAsset) (*os.File, error) {
	if m.bundles.dir == "" || !safeComponent(d.ID) || !safeComponent(d.Revision) || !safeComponent(asset.ID) {
		return nil, errors.New("bundle unavailable")
	}
	f, err := os.Open(filepath.Join(m.bundlePath(d), asset.ID))
	if err != nil {
		return nil, errors.New("bundle unavailable")
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != asset.Size || info.ModTime().UnixNano() != asset.MTimeNS {
		f.Close()
		return nil, errors.New("bundle changed")
	}
	return f, nil
}

// CleanupBundles reconciles bounded Arc-owned artifacts after FK cascade/delete/restore/crash.
func (m *Manager) CleanupBundles(ctx context.Context) error {
	release, ok := m.store.BeginCatalogWork()
	if !ok {
		return nil
	}
	defer release()
	c := &m.bundles
	if c.dir == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	refs, err := m.store.DownloadBundleRefs(ctx)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(c.dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".stage-") {
			if info, e := entry.Info(); e == nil && time.Since(info.ModTime()) > time.Hour {
				_ = os.RemoveAll(filepath.Join(c.dir, name))
			}
			continue
		}
		if !safeComponent(name) || c.readers[name] > 0 || c.building[name] {
			continue
		}
		revision, ok := refs[name]
		if !ok {
			if err = os.RemoveAll(filepath.Join(c.dir, name)); err != nil {
				return err
			}
			continue
		}
		versions, e := os.ReadDir(filepath.Join(c.dir, name))
		if e != nil {
			return e
		}
		for _, v := range versions {
			if safeComponent(v.Name()) && v.Name() != revision {
				if err = os.RemoveAll(filepath.Join(c.dir, name, v.Name())); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

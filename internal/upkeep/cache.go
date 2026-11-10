// Package upkeep removes only identified Arc-generated cache artifacts, never library media,
// backups, uploaded originals, certificates, logs or arbitrary files in the data directory.
package upkeep

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ams/internal/artwork"
	"ams/internal/store"
)

var hash = regexp.MustCompile(`^[a-f0-9]{24}$`)
var shard = regexp.MustCompile(`^[a-f0-9]{2}$`)
var sourceRevision = regexp.MustCompile(`^v2-([a-f0-9]{32})$`)
var sheet = regexp.MustCompile(`^\d+\.jpg$`)
var optimized = regexp.MustCompile(`^(\d+)-(720|1080)(-[A-Za-z0-9]+)?\.mp4(\.part)?$`)
var download = regexp.MustCompile(`^[a-z]{2,3}(\.forced)?(\.sdh)?\.\d+\.srt$`)

type CacheResult struct {
	Files  int
	Bytes  int64
	Legacy int
}

func (r CacheResult) String() string {
	detail := fmt.Sprintf("Removed %d unused cached files; reclaimed %s", r.Files, FormatBytes(r.Bytes))
	if r.Legacy > 0 {
		detail += fmt.Sprintf("; kept %d older subtitle caches with unknown ownership", r.Legacy)
	}
	return detail
}

func CleanCaches(ctx context.Context, st *store.Store, dataDir string, allowed func() bool) (result CacheResult, err error) {
	images, err := st.ItemImages(ctx)
	if err != nil {
		return result, err
	}
	keepArt := map[string]bool{}
	for _, im := range images {
		for _, src := range artwork.Sources(im.Kind, im.Type, im.Path) {
			if strings.HasPrefix(src, "http") {
				keepArt[artwork.Key(src)] = true
			}
		}
	}
	versions, err := st.OptimizedVersions(ctx)
	if err != nil {
		return result, err
	}
	keepOptimized := map[string]bool{}
	for _, o := range versions {
		if o.Path != "" {
			keepOptimized[filepath.Clean(o.Path)] = true
		}
	}
	used := map[int64]bool{}
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if allowed != nil && !allowed() {
			return context.Canceled
		}
		return nil
	}
	for _, kind := range []string{"artwork", "trickplay", "videothumbs", "subtitles", "optimized"} {
		if err := check(); err != nil {
			return result, err
		}
		dir := filepath.Join(dataDir, kind)
		info, err := os.Lstat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return result, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return result, fmt.Errorf("cache folder %s is not a regular directory; cleanup refused", kind)
		}
		root, err := os.OpenRoot(dir)
		if err != nil {
			return result, err
		}
		var emptyCandidates []string
		err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := check(); err != nil {
				return err
			}
			if d.IsDir() {
				if path != "." {
					emptyCandidates = append(emptyCandidates, path)
				}
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			parts := strings.Split(path, "/")
			id := int64(0)
			identified := false
			assetKey := ""
			switch kind {
			case "artwork":
				key := strings.Split(d.Name(), ".")[0]
				identified = len(parts) == 2 && hash.MatchString(key) && parts[0] == key[:2] && (d.Name() == key || strings.HasSuffix(d.Name(), ".tmp"))
				if keepArt[key] {
					return nil
				}
			case "trickplay":
				if len(parts) == 3 && sourceRevision.MatchString(parts[1]) && (d.Name() == "info.json" || d.Name() == "thumbs.bif" || sheet.MatchString(d.Name())) {
					id = cacheFileID(parts[0])
					identified = id > 0
					assetKey = strings.TrimPrefix(parts[1], "v2-")
				}
				if len(parts) == 2 && (d.Name() == "info.json" || sheet.MatchString(d.Name())) {
					id = cacheFileID(strings.TrimSuffix(parts[0], ".tmp"))
					identified = id > 0
				}
			case "videothumbs":
				if len(parts) == 1 && sheet.MatchString(d.Name()) {
					id = cacheFileID(strings.TrimSuffix(d.Name(), ".jpg"))
					identified = id > 0
				}
			case "subtitles":
				if len(parts) == 4 && parts[0] == "downloaded" && sourceRevision.MatchString(parts[2]) && download.MatchString(d.Name()) {
					id = cacheFileID(parts[1])
					identified = id > 0
					assetKey = strings.TrimPrefix(parts[2], "v2-")
				}
				if len(parts) == 1 && strings.HasSuffix(d.Name(), ".json") && hash.MatchString(strings.TrimSuffix(d.Name(), ".json")) {
					result.Legacy++
					return nil
				}
				if len(parts) == 2 && hash.MatchString(strings.TrimSuffix(d.Name(), ".json")) && strings.HasSuffix(d.Name(), ".json") {
					id = cacheFileID(parts[0])
					identified = id > 0
				}
				if len(parts) == 3 && parts[0] == "downloaded" && download.MatchString(d.Name()) {
					id = cacheFileID(parts[1])
					identified = id > 0
				}
			case "optimized":
				if keepOptimized[filepath.Clean(filepath.Join(dir, filepath.FromSlash(path)))] {
					return nil
				}
				if match := optimized.FindStringSubmatch(d.Name()); len(parts) == 1 && match != nil {
					id = cacheFileID(match[1])
					identified = id > 0
				}
			}
			if !identified || time.Since(info.ModTime()) < time.Hour {
				return nil
			}
			if id > 0 && assetKey != "" {
				current, err := st.FileCacheKey(ctx, id)
				if err != nil {
					return err
				}
				if current == assetKey {
					return nil
				}
			} else if id > 0 {
				if used[id] {
					return nil
				}
				present, err := st.HasMediaFile(ctx, id)
				if err != nil {
					return err
				}
				if present {
					used[id] = true
					return nil
				}
			}
			if err := check(); err != nil {
				return err
			}
			if err := root.Remove(path); err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			result.Files++
			result.Bytes += info.Size()
			return nil
		})
		if err == nil {
			// Remove only empty generated containers. Unknown files/symlinks keep their parent.
			for i := len(emptyCandidates) - 1; i >= 0; i-- {
				path := emptyCandidates[i]
				parts := strings.Split(path, "/")
				owned := false
				var id int64
				assetKey := ""
				if kind == "trickplay" && len(parts) == 2 && sourceRevision.MatchString(parts[1]) {
					id = cacheFileID(parts[0])
					owned = id > 0
					assetKey = strings.TrimPrefix(parts[1], "v2-")
				}
				if kind == "subtitles" && len(parts) == 3 && parts[0] == "downloaded" && sourceRevision.MatchString(parts[2]) {
					id = cacheFileID(parts[1])
					owned = id > 0
					assetKey = strings.TrimPrefix(parts[2], "v2-")
				}
				if kind == "artwork" && len(parts) == 1 && shard.MatchString(path) {
					owned = true
				}
				if (kind == "trickplay" || kind == "subtitles") && len(parts) == 1 {
					name := path
					if kind == "trickplay" {
						name = strings.TrimSuffix(name, ".tmp")
					}
					id = cacheFileID(name)
					owned = id > 0
				}
				if kind == "subtitles" && len(parts) == 2 && parts[0] == "downloaded" {
					id = cacheFileID(parts[1])
					owned = id > 0
				}
				if !owned {
					continue
				}
				if err = check(); err != nil {
					break
				}
				if id > 0 && assetKey != "" {
					current, e := st.FileCacheKey(ctx, id)
					if e != nil {
						err = e
						break
					}
					if current == assetKey {
						continue
					}
				} else if id > 0 {
					var present bool
					present, err = st.HasMediaFile(ctx, id)
					if err != nil {
						break
					}
					if present {
						continue
					}
				}
				var entries []fs.DirEntry
				entries, err = fs.ReadDir(root.FS(), path)
				if os.IsNotExist(err) {
					err = nil
					continue
				}
				if err != nil {
					break
				}
				if len(entries) != 0 {
					continue
				}
				if err = root.Remove(path); err != nil {
					if os.IsNotExist(err) {
						err = nil
						continue
					}
					break
				}
			}
		}
		root.Close()
		if err != nil {
			return result, fmt.Errorf("cleaning %s: %w", kind, err)
		}
	}
	return result, nil
}

func FormatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	i := 0
	for value >= 1024 && i < len(units)-1 {
		value /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", value, units[i])
}

func cacheFileID(name string) int64 {
	id, err := strconv.ParseInt(name, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != name {
		return 0
	}
	return id
}

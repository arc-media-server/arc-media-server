package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ams/internal/store"
)

// Other videos libraries: home videos and anything else with no online details. Each file is its
// own title, named from the file name and dated by a date in the name or the file's own date; its
// picture is a frame from the video, and each folder is a collection.

// videoFolder collects one folder's videos during a scan.
type videoFolder struct {
	items []videoEntry
}

type videoEntry struct {
	id, date, title string
}

var (
	// A date in a file name: 2019-07-14, 2019_07_14, 20190714 (and IMG_20190714_...).
	nameDateRe = regexp.MustCompile(`(?:^|\D)((?:19|20)\d{2})[-_. ]?(0[1-9]|1[0-2])[-_. ]?(0[1-9]|[12]\d|3[01])(?:\D|$)`)
	spacesRe   = regexp.MustCompile(`[\s._]+`)
)

// videoTitle turns a file name into a title: no extension, dots and underscores as spaces.
func videoTitle(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if t := strings.TrimSpace(spacesRe.ReplaceAllString(base, " ")); t != "" {
		return t
	}
	return base
}

// videoDate is the date a video was made: one in its name, or else the file's modification date.
func videoDate(path string, mod time.Time) string {
	if m := nameDateRe.FindStringSubmatch(filepath.Base(path)); m != nil {
		d := m[1] + "-" + m[2] + "-" + m[3]
		if _, err := time.Parse(time.DateOnly, d); err == nil {
			return d
		}
	}
	return mod.Format(time.DateOnly)
}

func (s *Scanner) scanVideoFile(ctx context.Context, lib *store.Library, root, path string, d fs.DirEntry, scanStart int64,
	folders map[string]*videoFolder) error {
	fileID, err := s.recordFile(ctx, lib, path, d, scanStart)
	if err != nil {
		return err
	}
	info, err := d.Info()
	if err != nil {
		return err
	}
	itemID, err := s.store.FileItemOverride(ctx, fileID)
	if err != nil {
		return err
	}
	if itemID == "" {
		var created bool
		itemID, created, err = s.store.EnsureItem(ctx, &store.Item{LibraryID: lib.ID, Kind: "movie", Title: videoTitle(path)},
			"file:"+strings.ToLower(path))
		if err != nil {
			return err
		}
		if created {
			s.update(lib.ID, func(st *Status) { st.NewItems++ })
		}
	}
	if err := s.store.SetFileItems(ctx, fileID, []string{itemID}, 0); err != nil {
		return err
	}
	date := videoDate(path, info.ModTime())
	title := videoTitle(path)
	if err := s.store.SetLocalVideo(ctx, itemID, title, date); err != nil {
		return err
	}
	if thumb := s.videoThumb(ctx, fileID, path); thumb != "" {
		if err := s.store.SetLocalArt(ctx, itemID, thumb, thumb, ""); err != nil {
			return err
		}
	}
	// Videos in a folder (not the library's own) make a collection named after it.
	if rel, err := filepath.Rel(root, filepath.Dir(path)); err == nil && rel != "." {
		f := folders[rel]
		if f == nil {
			f = &videoFolder{}
			folders[rel] = f
		}
		f.items = append(f.items, videoEntry{id: itemID, date: date, title: title})
	}
	return nil
}

// syncVideoFolders saves each folder's videos, oldest first, as a collection.
func (s *Scanner) syncVideoFolders(ctx context.Context, lib *store.Library, folders map[string]*videoFolder, complete bool) error {
	out := map[string][]string{}
	for rel, f := range folders {
		sort.SliceStable(f.items, func(i, j int) bool {
			if f.items[i].date != f.items[j].date {
				return f.items[i].date < f.items[j].date
			}
			return f.items[i].title < f.items[j].title
		})
		for _, e := range f.items {
			out[rel] = append(out[rel], e.id)
		}
	}
	return s.store.SyncFolderCollections(ctx, lib.ID, out, complete)
}

// videoThumb returns a picture for a video, a frame a tenth of the way in, made with FFmpeg the
// first time ("" when it can't be made).
func (s *Scanner) videoThumb(ctx context.Context, fileID int64, path string) string {
	if s.ThumbDir == "" || s.FFmpeg == nil {
		return ""
	}
	out := filepath.Join(s.ThumbDir, strconv.FormatInt(fileID, 10)+".jpg")
	if _, err := os.Stat(out); err == nil {
		return out
	}
	ffmpeg := s.FFmpeg()
	if ffmpeg == "" {
		return ""
	}
	f, err := s.store.GetFileByPath(ctx, path)
	if err != nil || f == nil {
		return ""
	}
	at := min(f.DurationMS/10, 5*60*1000)
	if err := os.MkdirAll(s.ThumbDir, 0o755); err != nil {
		return ""
	}
	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tmp := out + ".tmp.jpg"
	cmd := exec.CommandContext(tctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-ss", fmt.Sprintf("%.3f", float64(at)/1000),
		"-i", path, "-frames:v", "1", "-vf", "scale='min(960,iw)':-2", "-q:v", "3", "-y", tmp)
	if err := cmd.Run(); err != nil {
		os.Remove(tmp)
		s.log.Debug("couldn't make a video thumbnail", "path", path, "err", err)
		return ""
	}
	if err := os.Rename(tmp, out); err != nil {
		return ""
	}
	return out
}

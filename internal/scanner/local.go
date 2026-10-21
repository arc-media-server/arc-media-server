package scanner

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// Local media assets: artwork and Kodi-style .nfo files that people (or Radarr/Sonarr) keep next to
// their media. AMS only reads them.

var imageExts = []string{".jpg", ".jpeg", ".png", ".webp"}

var imdbRe = regexp.MustCompile(`tt\d{7,9}`)

// Enumerate once rather than issuing dozens of failed Stat calls for absent assets.
func imageFinder(dir string) func(...string) string {
	entries, _ := os.ReadDir(dir)
	names := map[string]os.DirEntry{}
	key := func(name string) string {
		if runtime.GOOS == "windows" {
			return strings.ToLower(name)
		}
		return name
	}
	for _, e := range entries {
		names[key(e.Name())] = e
	}
	return func(bases ...string) string {
		for _, b := range bases {
			for _, ext := range imageExts {
				if e := names[key(b+ext)]; e != nil {
					if info, err := e.Info(); err == nil && !info.IsDir() && info.Size() > 0 {
						return filepath.Join(dir, e.Name())
					}
				}
			}
		}
		return ""
	}
}

// themeExts are the audio files a theme tune can be.
var themeExts = []string{".mp3", ".flac", ".m4a", ".ogg", ".opus", ".wav"}

// findTheme returns a folder's theme tune: theme.mp3 (Plex, Kodi) or the first tune in a
// theme-music folder (Jellyfin), or "".
func findTheme(dir string) string {
	for _, ext := range themeExts {
		p := filepath.Join(dir, "theme"+ext)
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Size() > 0 {
			return p
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "theme-music"))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && slices.Contains(themeExts, strings.ToLower(filepath.Ext(e.Name()))) {
			return filepath.Join(dir, "theme-music", e.Name())
		}
	}
	return ""
}

// LocalArt is artwork found next to the media.
type LocalArt struct {
	Poster, Backdrop, Logo string
	// A theme tune, played quietly on the title's page.
	Theme string
	// Season posters (shows): season number -> file.
	Seasons map[int]string
}

// movieArt finds a movie's local artwork: "<file name>-poster.jpg" beside the file, or poster.jpg
// and friends in its folder when the movie has a folder of its own.
func movieArt(path string, ownFolder bool) LocalArt {
	dir := filepath.Dir(path)
	image := imageFinder(dir)
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	art := LocalArt{
		Poster:   image(stem+"-poster", stem+".poster"),
		Backdrop: image(stem+"-fanart", stem+"-backdrop", stem+".fanart"),
		Logo:     image(stem+"-clearlogo", stem+"-logo"),
	}
	if ownFolder {
		art.Theme = findTheme(dir)
		if art.Poster == "" {
			art.Poster = image("poster", "folder", "cover", "movie")
		}
		if art.Backdrop == "" {
			art.Backdrop = image("fanart", "backdrop", "background", "art")
		}
		if art.Logo == "" {
			art.Logo = image("clearlogo", "logo")
		}
	}
	return art
}

// seriesArt finds a show's local artwork in its folder.
func seriesArt(dir string) LocalArt {
	image := imageFinder(dir)
	return LocalArt{
		Poster:   image("poster", "folder", "cover", "show"),
		Backdrop: image("fanart", "backdrop", "background", "art"),
		Logo:     image("clearlogo", "logo"),
		Seasons:  seasonPosters(dir),
		Theme:    findTheme(dir),
	}
}

// seasonPosterName matches season poster files in a show's folder, as Plex, Kodi and Jellyfin
// name them: season01-poster.jpg, season1-poster.png, season-specials-poster.jpg.
var seasonPosterName = regexp.MustCompile(`(?i)^season(\d+|-specials)-poster\.(jpe?g|png|webp)$`)

// seasonPosters finds season poster files in a show's folder.
func seasonPosters(dir string) map[int]string {
	out := map[int]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		m := seasonPosterName.FindStringSubmatch(e.Name())
		if m == nil || e.IsDir() {
			continue
		}
		n := 0
		if m[1] != "-specials" {
			n, _ = strconv.Atoi(m[1])
		}
		if _, dup := out[n]; !dup {
			out[n] = filepath.Join(dir, e.Name())
		}
	}
	return out
}

// NFO is what AMS uses from a Kodi-style .nfo file: the ids that identify the title exactly.
type NFO struct {
	TMDBID int64
	TVDBID int64
	IMDBID string
}

func (n NFO) empty() bool { return n.TMDBID == 0 && n.TVDBID == 0 && n.IMDBID == "" }

// readNFO reads ids from an .nfo file (movie, tvshow or episodedetails). Files that aren't XML
// (some tools write just an IMDb link) are searched for an IMDb id.
func readNFO(path string) NFO {
	f, err := os.Open(path)
	if err != nil {
		return NFO{}
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 1<<20))
	if err != nil {
		return NFO{}
	}
	var doc struct {
		ID        string `xml:"id"`
		TMDBID    string `xml:"tmdbid"`
		TVDBID    string `xml:"tvdbid"`
		IMDBID    string `xml:"imdbid"`
		UniqueIDs []struct {
			Type  string `xml:"type,attr"`
			Value string `xml:",chardata"`
		} `xml:"uniqueid"`
	}
	var n NFO
	if xml.Unmarshal(raw, &doc) == nil {
		set := func(kind, v string) {
			v = strings.TrimSpace(v)
			switch strings.ToLower(kind) {
			case "tmdb":
				if id, err := strconv.ParseInt(v, 10, 64); err == nil && n.TMDBID == 0 {
					n.TMDBID = id
				}
			case "tvdb":
				if id, err := strconv.ParseInt(v, 10, 64); err == nil && n.TVDBID == 0 {
					n.TVDBID = id
				}
			case "imdb":
				if strings.HasPrefix(v, "tt") && n.IMDBID == "" {
					n.IMDBID = v
				}
			}
		}
		for _, u := range doc.UniqueIDs {
			set(u.Type, u.Value)
		}
		set("tmdb", doc.TMDBID)
		set("tvdb", doc.TVDBID)
		set("imdb", doc.IMDBID)
		set("imdb", doc.ID)
	}
	if n.IMDBID == "" {
		if m := imdbRe.FindString(string(raw)); m != "" {
			n.IMDBID = m
		}
	}
	return n
}

// movieNFO finds a movie's .nfo: "<file name>.nfo" beside it, or movie.nfo in its own folder.
func movieNFO(path string, ownFolder bool) NFO {
	stem := strings.TrimSuffix(path, filepath.Ext(path))
	if n := readNFO(stem + ".nfo"); !n.empty() {
		return n
	}
	if ownFolder {
		return readNFO(filepath.Join(filepath.Dir(path), "movie.nfo"))
	}
	return NFO{}
}

package scanner

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Local media assets: artwork and Kodi-style .nfo files that people (or Radarr/Sonarr) keep next to
// their media. AMS only reads them.

var imageExts = []string{".jpg", ".jpeg", ".png", ".webp"}

var imdbRe = regexp.MustCompile(`tt\d{7,9}`)

// findImage returns the first existing image named base + one of the image extensions in dir.
func findImage(dir string, bases ...string) string {
	for _, b := range bases {
		for _, ext := range imageExts {
			p := filepath.Join(dir, b+ext)
			if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Size() > 0 {
				return p
			}
		}
	}
	return ""
}

// LocalArt is artwork found next to the media.
type LocalArt struct {
	Poster, Backdrop, Logo string
}

// movieArt finds a movie's local artwork: "<file name>-poster.jpg" beside the file, or poster.jpg
// and friends in its folder when the movie has a folder of its own.
func movieArt(path string, ownFolder bool) LocalArt {
	dir := filepath.Dir(path)
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	art := LocalArt{
		Poster:   findImage(dir, stem+"-poster", stem+".poster"),
		Backdrop: findImage(dir, stem+"-fanart", stem+"-backdrop", stem+".fanart"),
		Logo:     findImage(dir, stem+"-clearlogo", stem+"-logo"),
	}
	if ownFolder {
		if art.Poster == "" {
			art.Poster = findImage(dir, "poster", "folder", "cover", "movie")
		}
		if art.Backdrop == "" {
			art.Backdrop = findImage(dir, "fanart", "backdrop", "background", "art")
		}
		if art.Logo == "" {
			art.Logo = findImage(dir, "clearlogo", "logo")
		}
	}
	return art
}

// seriesArt finds a show's local artwork in its folder.
func seriesArt(dir string) LocalArt {
	return LocalArt{
		Poster:   findImage(dir, "poster", "folder", "cover", "show"),
		Backdrop: findImage(dir, "fanart", "backdrop", "background", "art"),
		Logo:     findImage(dir, "clearlogo", "logo"),
	}
}

// episodeThumb finds an episode's local still: "<file name>-thumb.jpg" beside the file.
func episodeThumb(path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return findImage(filepath.Dir(path), stem+"-thumb", stem)
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

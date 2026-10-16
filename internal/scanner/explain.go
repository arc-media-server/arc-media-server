package scanner

import (
	"path/filepath"
	"strings"

	"ams/internal/store"
)

// ParsedName is what AMS read from a file's path, for explaining a match.
type ParsedName struct {
	Path string `json:"path"`
	// The part of the path the title was read from (a file or folder name).
	Source     string `json:"source"`
	SourceKind string `json:"source_kind"` // file or folder
	Title      string `json:"title"`
	Year       int    `json:"year,omitempty"`
	TMDBID     int64  `json:"tmdb_id,omitempty"`
	TVDBID     int64  `json:"tvdb_id,omitempty"`
	IMDBID     string `json:"imdb_id,omitempty"`
	// Episodes only: the episode marker found, in the library's source ordering.
	Season   *int   `json:"season,omitempty"`
	Episodes []int  `json:"episodes,omitempty"`
	Absolute bool   `json:"absolute,omitempty"`
	AirDate  string `json:"air_date,omitempty"`
	// False when the file name couldn't be read as an episode at all.
	Recognised bool `json:"recognised"`
}

// ExplainMovieFile reports how a movie file's path was read.
func ExplainMovieFile(path string) ParsedName {
	n := ParseMovie(path)
	p := ParsedName{Path: path, Title: n.Title, Year: n.Year, TMDBID: n.TMDBID, IMDBID: n.IMDBID,
		Source: filepath.Base(path), SourceKind: "file", Recognised: n.Title != ""}
	if n.FromFolder {
		p.Source, p.SourceKind = filepath.Base(filepath.Dir(path)), "folder"
	}
	return p
}

// ExplainEpisodeFile reports how an episode file's path was read, given the library's folders.
func ExplainEpisodeFile(lib *store.Library, path string) ParsedName {
	p := ParsedName{Path: path, Source: filepath.Base(path), SourceKind: "file"}
	root := ""
	for _, r := range lib.Paths {
		if rel, err := filepath.Rel(r, path); err == nil && !strings.HasPrefix(rel, "..") && len(r) > len(root) {
			root = r
		}
	}
	if root == "" {
		return p
	}
	_, name, ep, ok := ParseEpisodePath(root, path)
	if !ok {
		return p
	}
	p.Recognised = true
	p.Title, p.Year, p.TMDBID, p.TVDBID, p.IMDBID = name.Title, name.Year, name.TMDBID, name.TVDBID, name.IMDBID
	if rel, _ := filepath.Rel(root, path); strings.ContainsAny(filepath.ToSlash(rel), "/") {
		p.Source, p.SourceKind = strings.Split(filepath.ToSlash(rel), "/")[0], "folder"
	}
	if ep.Season >= 0 {
		season := ep.Season
		p.Season = &season
	}
	p.Episodes, p.Absolute, p.AirDate = ep.Episodes, ep.Absolute, ep.AirDate
	return p
}

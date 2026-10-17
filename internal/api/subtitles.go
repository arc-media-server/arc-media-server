package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"ams/internal/opensubtitles"
	"ams/internal/store"
	"ams/internal/subtitles"
)

// subtitleTracks lists a file's subtitles, including ones downloaded for it.
func (s *Server) subtitleTracks(f *store.MediaFile) []subtitles.Track {
	if s.subtitles == nil {
		return subtitles.Tracks(f)
	}
	return s.subtitles.Tracks(f)
}

func (s *Server) findSubtitle(f *store.MediaFile, id string) (subtitles.Track, bool) {
	if s.subtitles == nil {
		return subtitles.Find(f, id)
	}
	return s.subtitles.Find(f, id)
}

// itemFile finds the item in the path and the file of it named by ?file= (or its first present
// file), writing an error if there's none.
func (s *Server) itemFile(w http.ResponseWriter, r *http.Request, fileID int64) (*store.Item, *store.MediaFile, bool) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return nil, nil, false
	}
	files, err := s.store.ItemFiles(r.Context(), it.ID)
	if err != nil {
		s.fail(w, err)
		return nil, nil, false
	}
	for i := range files {
		if !files[i].Missing && (fileID == 0 || files[i].ID == fileID) {
			return it, &files[i], true
		}
	}
	writeError(w, http.StatusNotFound, "no file for this item")
	return nil, nil, false
}

// listSubtitles: GET /api/items/{id}/subtitles?file=<id> lists a file's subtitle tracks.
func (s *Server) listSubtitles(w http.ResponseWriter, r *http.Request) {
	fileID, _ := strconv.ParseInt(r.URL.Query().Get("file"), 10, 64)
	_, f, ok := s.itemFile(w, r, fileID)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.subtitleTracks(f))
}

var episodeNumberRe = regexp.MustCompile(`(?i)s(\d{1,2})[ ._-]?e(\d{1,4})`)

// searchSubtitles: GET /api/items/{id}/subtitles/search?file=<id>&languages=en,fr finds subtitles
// online (OpenSubtitles) for a movie or episode, best first.
func (s *Server) searchSubtitles(w http.ResponseWriter, r *http.Request) {
	if s.subsOnline == nil || !s.subsOnline.Configured() {
		writeError(w, http.StatusServiceUnavailable, opensubtitles.ErrNoKey.Error())
		return
	}
	fileID, _ := strconv.ParseInt(r.URL.Query().Get("file"), 10, 64)
	it, f, ok := s.itemFile(w, r, fileID)
	if !ok {
		return
	}
	q := opensubtitles.Query{TMDBID: it.TMDBID, IMDBID: it.IMDBID, Title: it.Title, Year: it.Year}
	for _, l := range strings.Split(r.URL.Query().Get("languages"), ",") {
		if l = strings.TrimSpace(l); l != "" {
			if two := subtitles.LanguageCode2(l); two != "" {
				q.Languages = append(q.Languages, two)
			} else {
				q.Languages = append(q.Languages, l)
			}
		}
	}
	if it.Kind == "episode" {
		series, err := s.store.GetItem(r.Context(), it.ParentID)
		if err != nil || series == nil {
			writeError(w, http.StatusNotFound, "this episode's show wasn't found")
			return
		}
		q.TMDBID, q.IMDBID = 0, ""
		q.ParentTMDBID, q.ParentIMDBID, q.Title = series.TMDBID, series.IMDBID, series.Title
		// The numbers in the file's name, which match how OpenSubtitles numbers episodes.
		if m := episodeNumberRe.FindStringSubmatch(f.Path); m != nil {
			q.Season, _ = strconv.Atoi(m[1])
			q.Episode, _ = strconv.Atoi(m[2])
		}
	}
	if h, err := opensubtitles.Hash(f.Path); err == nil {
		q.Hash = h
	}
	results, err := s.subsOnline.Search(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// downloadSubtitles: POST /api/items/{id}/subtitles/download {"file": <id>, "result": <file_id>,
// "language", "forced", "hearing_impaired"} downloads a subtitle found online and keeps it in
// Arc's data folder (never beside the video), returning the new track.
func (s *Server) downloadSubtitles(w http.ResponseWriter, r *http.Request) {
	var body struct {
		File            int64  `json:"file"`
		Result          int64  `json:"result"`
		Language        string `json:"language"`
		Forced          bool   `json:"forced"`
		HearingImpaired bool   `json:"hearing_impaired"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Result == 0 {
		writeError(w, http.StatusBadRequest, `send {"file", "result", "language"}`)
		return
	}
	if s.subsOnline == nil || s.subtitles == nil {
		writeError(w, http.StatusServiceUnavailable, opensubtitles.ErrNoKey.Error())
		return
	}
	_, f, ok := s.itemFile(w, r, body.File)
	if !ok {
		return
	}
	data, _, remaining, err := s.subsOnline.Download(r.Context(), body.Result)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	t, err := s.subtitles.SaveDownload(f, body.Language, body.Forced, body.HearingImpaired, data)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("downloaded subtitles", "file", f.Path, "language", body.Language, "remaining_today", remaining)
	writeJSON(w, http.StatusOK, map[string]any{"track": t, "remaining": remaining})
}

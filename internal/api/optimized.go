package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"ams/internal/optimize"
	"ams/internal/store"
)

// optimizeItems: POST /api/optimize {"item_ids": [...], "quality": 1080} (admin) queues optimized
// copies: a movie's main version, an episode's file, or every episode of a show.
func (s *Server) optimizeItems(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ItemIDs []string `json:"item_ids"`
		Quality int      `json:"quality"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.ItemIDs) == 0 {
		writeError(w, http.StatusBadRequest, `send {"item_ids": [...], "quality": 1080 or 720}`)
		return
	}
	if _, ok := optimize.Qualities[body.Quality]; !ok {
		writeError(w, http.StatusBadRequest, "quality is 1080 or 720")
		return
	}
	ctx := r.Context()
	var fileIDs []int64
	var selected []store.MediaFile
	addFiles := func(itemID string) error {
		files, err := s.store.ItemFiles(ctx, itemID)
		if err != nil {
			return err
		}
		// The best version: the most lines tall, then the biggest file.
		var best *store.MediaFile
		bestHeight := -1
		for i := range files {
			f := &files[i]
			if f.Missing || f.Part > 1 {
				continue
			}
			h := 0
			if v := videoStream(f); v != nil {
				h = v.Height
			}
			if h > bestHeight || (h == bestHeight && f.Size > best.Size) {
				best, bestHeight = f, h
			}
		}
		if best != nil {
			fileIDs = append(fileIDs, best.ID)
			selected = append(selected, *best)
		}
		return nil
	}
	for _, id := range body.ItemIDs {
		it, err := s.store.GetItem(ctx, id)
		if err != nil {
			s.fail(w, err)
			return
		}
		if it == nil {
			continue
		}
		if it.Kind == "series" {
			eps, err := s.store.Children(ctx, it.ID)
			if err != nil {
				s.fail(w, err)
				return
			}
			for _, e := range eps {
				if err := addFiles(e.ID); err != nil {
					s.fail(w, err)
					return
				}
			}
			continue
		}
		if err := addFiles(it.ID); err != nil {
			s.fail(w, err)
			return
		}
	}
	// An explicit Optimize can repair missing artifacts and changes not scanned yet.
	for _, f := range selected {
		list, err := s.store.FileOptimized(ctx, f.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		for _, o := range list {
			if o.Quality == body.Quality && !optimize.Usable(&o, &f) {
				if _, err := s.store.FailOptimize(ctx, &o, "The source or copy changed. Optimize it again."); err != nil {
					s.fail(w, err)
					return
				}
			}
		}
	}
	n, err := s.store.QueueOptimize(ctx, fileIDs, body.Quality)
	if err != nil {
		s.fail(w, err)
		return
	}
	if s.optimizer != nil {
		s.optimizer.Wake()
	}
	writeJSON(w, http.StatusOK, map[string]int{"queued": n})
}

// listOptimized: GET /api/optimized (admin) lists the optimized copies and the space they use.
func (s *Server) listOptimized(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.OptimizedVersions(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	var total int64
	for _, o := range list {
		total += o.Size
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": list, "total_size": total})
}

// deleteOptimized: DELETE /api/optimized/{file}/{quality} (admin) removes a copy.
func (s *Server) deleteOptimized(w http.ResponseWriter, r *http.Request) {
	fileID, err1 := strconv.ParseInt(r.PathValue("file"), 10, 64)
	quality, err2 := strconv.Atoi(r.PathValue("quality"))
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, "no such optimized version")
		return
	}
	if err := s.optimizer.Delete(r.Context(), fileID, quality); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// optimizedFor picks a finished optimized copy of a file this player can play as it is (MP4,
// H.264, AAC), within a height limit (0 for none), or nil.
func (s *Server) optimizedFor(r *http.Request, f *store.MediaFile, caps clientCaps, maxHeight, maxKbps int, audio *store.Stream) *store.Optimized {
	if caps.MaxBitDepth > 0 && caps.MaxBitDepth < 8 || caps.MaxAudioChannels > 0 && caps.MaxAudioChannels < 2 {
		return nil
	}
	if profiles, limited := caps.VideoProfiles["h264"]; limited && !slices.ContainsFunc(profiles, func(p string) bool { return strings.EqualFold(p, "High") }) {
		return nil
	}
	if !caps.has(caps.Containers, "mp4") || !caps.has(caps.Video, "h264") || !caps.has(caps.Audio, "aac") {
		return nil
	}
	list, err := s.store.FileOptimized(r.Context(), f.ID)
	if err != nil {
		return nil
	}
	for i := range list {
		if (maxHeight == 0 || list[i].Quality <= maxHeight) && list[i].WithinBitrate(maxKbps) && list[i].MatchesAudio(audio) && optimize.Usable(&list[i], f) {
			return &list[i]
		}
	}
	return nil
}

// optimizedURL is where a player streams an optimized copy.
func optimizedURL(itemID string, o *store.Optimized) string {
	return fmt.Sprintf("/api/items/%s/stream?file=%d&optimized=%d&optimized_rev=%s", itemID, o.FileID, o.Quality, o.JobID)
}

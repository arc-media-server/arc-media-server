package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"ams/internal/scanner"
	"ams/internal/store"
)

// editItems: POST /api/items/bulk (admin) changes many movies and shows at once:
//
//	{"item_ids": [...], "add_genres": [...], "remove_genres": [...], "add_labels": [...],
//	 "remove_labels": [...], "content_rating": "15", "locks": "all" | "none",
//	 "played": true, "refresh": true}
//
// Every field but item_ids is optional. "played" is for the current profile. "refresh" fetches
// their metadata again in the background (it shows in Activity).
func (s *Server) editItems(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ItemIDs       []string `json:"item_ids"`
		AddGenres     []string `json:"add_genres"`
		RemoveGenres  []string `json:"remove_genres"`
		AddLabels     []string `json:"add_labels"`
		RemoveLabels  []string `json:"remove_labels"`
		ContentRating *string  `json:"content_rating"`
		Locks         string   `json:"locks"`
		Played        *bool    `json:"played"`
		Refresh       bool     `json:"refresh"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	switch {
	case len(body.ItemIDs) == 0:
		writeError(w, http.StatusBadRequest, "item_ids lists the movies and shows to change")
		return
	case len(body.ItemIDs) > 5000:
		writeError(w, http.StatusBadRequest, "change at most 5,000 titles at once")
		return
	case body.Locks != "" && body.Locks != "all" && body.Locks != "none":
		writeError(w, http.StatusBadRequest, `locks is "all", "none" or left out`)
		return
	}
	ctx := r.Context()
	items := make([]*store.Item, 0, len(body.ItemIDs))
	for _, id := range body.ItemIDs {
		it, err := s.store.GetItem(ctx, id)
		if err != nil {
			s.fail(w, err)
			return
		}
		if it == nil || (it.Kind != "movie" && it.Kind != "series") {
			writeError(w, http.StatusBadRequest, "only movies and shows can be changed together: "+id)
			return
		}
		items = append(items, it)
	}
	e := store.BulkEdit{
		AddGenres: cleanLabels(body.AddGenres), RemoveGenres: cleanLabels(body.RemoveGenres),
		AddLabels: cleanLabels(body.AddLabels), RemoveLabels: cleanLabels(body.RemoveLabels),
		Locks: body.Locks,
	}
	if body.ContentRating != nil {
		v := strings.TrimSpace(*body.ContentRating)
		e.ContentRating = &v
	}
	if err := s.store.EditItems(ctx, body.ItemIDs, e); err != nil {
		s.fail(w, err)
		return
	}
	if body.Played != nil {
		for _, it := range items {
			ids := []string{it.ID}
			if it.Kind == "series" {
				var err error
				if ids, err = s.seriesEpisodeIDs(r, it, nil); err != nil {
					s.fail(w, err)
					return
				}
			}
			if err := s.store.SetPlayed(ctx, currentUserID(r), ids, *body.Played); err != nil {
				s.fail(w, err)
				return
			}
			if *body.Played && it.Kind == "movie" {
				if err := s.store.SetWatchlist(ctx, currentUserID(r), it.ID, false); err != nil {
					s.fail(w, err)
					return
				}
			}
		}
	}
	if body.Refresh {
		go s.refreshItems(context.WithoutCancel(ctx), items)
	}
	w.WriteHeader(http.StatusNoContent)
}

// refreshItems fetches metadata again for many titles, one at a time, reporting in Activity.
func (s *Server) refreshItems(ctx context.Context, items []*store.Item) {
	task := s.activity.Start("refresh", fmt.Sprintf("Refreshing %d titles", len(items)))
	failed := 0
	for i, it := range items {
		task.Progress(i+1, len(items))
		task.Detail(it.Title)
		var err error
		if it.Kind == "movie" {
			_, err = s.scanner.RefreshMovie(ctx, it.ID)
		} else {
			var lib *store.Library
			if lib, err = s.store.GetLibrary(ctx, it.LibraryID); err == nil && lib != nil {
				err = s.scanner.SyncSeriesMetadata(ctx, lib, it.ID, false)
			}
		}
		if err != nil {
			failed++
			s.log.Warn("refresh failed", "item", it.Title, "err", err)
		}
	}
	task.Detail(fmt.Sprintf("%d refreshed", len(items)-failed))
	var err error
	if failed > 0 {
		err = fmt.Errorf("%d of %d couldn't be refreshed (see the server log)", failed, len(items))
	}
	task.Finish(err)
}

// mergeItems: POST /api/items/merge {"target_id", "item_ids": [...]} (admin) makes several movies
// one, as versions of the target. Watch state, history and collections move with them.
func (s *Server) mergeItems(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetID string   `json:"target_id"`
		ItemIDs  []string `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	ctx := r.Context()
	target, err := s.store.GetItem(ctx, body.TargetID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if target == nil || target.Kind != "movie" {
		writeError(w, http.StatusBadRequest, "target_id is the movie to keep (only movies can be merged)")
		return
	}
	var sources []string
	for _, id := range body.ItemIDs {
		if id == target.ID {
			continue
		}
		it, err := s.store.GetItem(ctx, id)
		if err != nil {
			s.fail(w, err)
			return
		}
		if it == nil || it.Kind != "movie" || it.LibraryID != target.LibraryID {
			writeError(w, http.StatusBadRequest, "only movies in the same library can be merged: "+id)
			return
		}
		sources = append(sources, id)
	}
	if len(sources) == 0 {
		writeError(w, http.StatusBadRequest, "item_ids lists the movies to merge into the target")
		return
	}
	if err := s.store.MergeItems(ctx, target.ID, sources); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("titles merged", "into", target.Title, "count", len(sources))
	w.WriteHeader(http.StatusNoContent)
}

// splitItem: POST /api/items/{id}/split (admin) gives each version of a movie but the first its own
// title; they're identified at the scan that starts straight after.
func (s *Server) splitItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	if it.Kind != "movie" {
		writeError(w, http.StatusBadRequest, "only movies with several versions can be split apart")
		return
	}
	ids, err := s.store.SplitItem(r.Context(), it, func(path string) (string, int) {
		n := scanner.ParseMovie(path)
		return n.Title, n.Year
	})
	if err != nil {
		s.fail(w, err)
		return
	}
	if len(ids) == 0 {
		writeError(w, http.StatusBadRequest, "this movie has only one version")
		return
	}
	s.log.Info("title split apart", "title", it.Title, "new", len(ids))
	s.scanner.Start(it.LibraryID)
	writeJSON(w, http.StatusOK, map[string]any{"item_ids": ids})
}

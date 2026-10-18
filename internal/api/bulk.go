package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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

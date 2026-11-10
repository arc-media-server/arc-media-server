package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
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
	if body.Played != nil {
		watch := &store.BulkWatch{UserID: currentUserID(r), Played: *body.Played}
		seen := make(map[string]bool)
		for _, it := range items {
			ids := []string{it.ID}
			if it.Kind == "series" {
				var err error
				if ids, err = s.seriesEpisodeIDs(r, it, nil); err != nil {
					s.fail(w, err)
					return
				}
			} else if *body.Played {
				watch.RemoveWatchlist = append(watch.RemoveWatchlist, it.ID)
			}
			for _, id := range ids {
				if !seen[id] {
					watch.ItemIDs = append(watch.ItemIDs, id)
					seen[id] = true
				}
			}
		}
		e.Watch = watch
	}
	if err := s.store.EditItems(ctx, body.ItemIDs, e); err != nil {
		s.fail(w, err)
		return
	}
	if body.Refresh {
		go s.refreshItems(context.WithoutCancel(ctx), items)
	}
	w.WriteHeader(http.StatusNoContent)
}

// refreshItems fetches metadata again for many titles, one at a time, reporting in Activity.
func (s *Server) refreshItems(ctx context.Context, items []*store.Item) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	task := s.activity.Start("refresh", fmt.Sprintf("Refreshing %d titles", len(items)))
	task.Cancellable(cancel)
	failed := 0
	for i, it := range items {
		if ctx.Err() != nil {
			task.Finish(ctx.Err())
			return
		}
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
		if ctx.Err() != nil {
			task.Finish(ctx.Err())
			return
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
// one, as versions of the target, or several shows one (store.MergeSeries), all from one library.
// Watch state, history and collections move with them. Shows must number their episode files the
// same way; merging them holds the library's work slot, then refreshes the kept show.
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
	if target == nil || (target.Kind != "movie" && target.Kind != "series") {
		writeError(w, http.StatusBadRequest, "target_id is the movie or show to keep")
		return
	}
	lib, err := s.store.GetLibrary(ctx, target.LibraryID)
	if err != nil || lib == nil {
		s.fail(w, cmp.Or(err, errors.New("the library is gone")))
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
		if it == nil || it.Kind != target.Kind || it.LibraryID != target.LibraryID {
			writeError(w, http.StatusBadRequest, "only "+kindPlural(target.Kind)+" in the same library can be merged: "+id)
			return
		}
		if target.Kind == "series" && numberingProvider(it, lib) != numberingProvider(target, lib) {
			writeError(w, http.StatusBadRequest, it.Title+" numbers its episode files differently ("+
				numberingProvider(it, lib)+"). Set both shows' File episode order the same in Edit, then merge.")
			return
		}
		sources = append(sources, id)
	}
	if len(sources) == 0 {
		writeError(w, http.StatusBadRequest, "item_ids lists the titles to merge into the target")
		return
	}
	if target.Kind == "series" {
		// Merge under the library's work slot, then place every episode in the kept show's orders.
		err := s.scanner.RefreshSeries(ctx, lib, target.ID, false, func(ctx context.Context) error {
			return s.store.MergeSeries(ctx, target.ID, sources)
		})
		if errors.Is(err, scanner.ErrLibraryBusy) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		if err != nil {
			// The merge itself happened if the refresh is what failed; say so plainly.
			if merged, _ := s.store.GetItem(ctx, sources[0]); merged == nil {
				s.log.Warn("shows merged, but refreshing the kept show failed", "into", target.Title, "err", err)
				writeError(w, http.StatusBadGateway, "the shows were merged, but refreshing "+target.Title+" failed: "+err.Error()+". Use Refresh metadata to try again.")
				return
			}
			s.fail(w, err)
			return
		}
	} else if err := s.store.MergeItems(ctx, target.ID, sources); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("titles merged", "into", target.Title, "kind", target.Kind, "count", len(sources))
	w.WriteHeader(http.StatusNoContent)
}

func kindPlural(kind string) string {
	if kind == "series" {
		return "shows"
	}
	return "movies"
}

// numberingProvider is whose numbering a show's episode files follow (tvdb or tmdb): its own
// File episode order, else its library's.
func numberingProvider(it *store.Item, lib *store.Library) string {
	order := cmp.Or(it.SourceOrdering, lib.SourceOrdering, "tvdb_aired")
	provider, _, _ := strings.Cut(order, "_")
	return provider
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

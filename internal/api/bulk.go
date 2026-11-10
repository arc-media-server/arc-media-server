package api

import (
	"ams/internal/safeerr"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "bulk request exceeds 2 MiB")
			return
		}
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
	root := s.workContext
	if root == nil {
		root = context.Background()
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(root, cancel)
	defer stop()
	if root.Err() != nil {
		writeError(w, 503, "server is stopping; retry after it restarts")
		return
	}
	unique := make([]string, 0, len(body.ItemIDs))
	seen := map[string]bool{}
	for _, id := range body.ItemIDs {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	body.ItemIDs = unique
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
	var ticket *bulkRefreshTicket
	if body.Refresh {
		if s.scanner == nil {
			writeError(w, 503, "metadata refresh is unavailable")
			return
		}
		var err error
		ticket, err = s.bulkRefreshWork().reserve(body.ItemIDs)
		if err != nil {
			if errors.Is(err, errBulkRefreshFull) {
				w.Header().Set("Retry-After", "5")
				writeError(w, 429, "metadata refresh queue is full; retry when current work finishes")
			} else {
				w.Header().Set("Retry-After", "3")
				writeError(w, 503, "metadata refresh is temporarily unavailable; retry shortly")
			}
			return
		}
		defer ticket.abort()
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
	if ticket != nil {
		ticket.commit()
	}
	w.WriteHeader(http.StatusNoContent)
}

// refreshItems fetches a queue batch with fresh item identities and one bounded outcome.
func (s *Server) refreshItems(ctx context.Context, ids []string, complete func(string)) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	task := s.activity.Start("refresh", fmt.Sprintf("Refreshing %d titles", len(ids)))
	task.Cancellable(cancel)
	started := time.Now()
	failed, refreshed, skipped := 0, 0, 0
	firstFailure := ""
	s.log.Info("bulk metadata refresh started", "titles", len(ids))
	defer func() {
		for _, id := range ids {
			complete(id)
		}
		err := ctx.Err()
		if failed > 0 && err == nil {
			err = fmt.Errorf("%d of %d could not be refreshed", failed, len(ids))
		}
		task.Detail(fmt.Sprintf("%d refreshed, %d skipped, %d failed", refreshed, skipped, failed))
		task.Finish(err)
		if failed > 0 && ctx.Err() == nil {
			s.log.Warn("bulk metadata refresh finished with failures", "requested", len(ids), "refreshed", refreshed, "skipped", skipped, "failed", failed, "took", time.Since(started), "first_error", firstFailure)
		} else {
			s.log.Info("bulk metadata refresh finished", "requested", len(ids), "refreshed", refreshed, "skipped", skipped, "failed", failed, "canceled", ctx.Err() != nil, "took", time.Since(started))
		}
	}()
	for i, id := range ids {
		if ctx.Err() != nil {
			return
		}
		task.Progress(i+1, len(ids))
		it, err := s.store.GetItem(ctx, id)
		if err == nil && it == nil {
			skipped++
			complete(id)
			continue
		}
		if err == nil {
			task.Detail(it.Title)
			if it.Kind == "movie" {
				_, err = s.scanner.RefreshMovie(ctx, it.ID)
			} else if it.Kind == "series" {
				var lib *store.Library
				lib, err = s.store.GetLibrary(ctx, it.LibraryID)
				if err == nil && lib != nil {
					err = s.scanner.SyncSeriesMetadata(ctx, lib, it.ID, false)
				} else if err == nil {
					skipped++
					complete(id)
					continue
				}
			} else {
				skipped++
				complete(id)
				continue
			}
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			failed++
			debug := s.log.Enabled(ctx, slog.LevelDebug)
			if firstFailure == "" || debug {
				message := err.Error()
				if it != nil {
					message = it.Title + ": " + message
				}
				encoded, _ := json.Marshal(map[string]string{"error": message})
				var safe map[string]string
				_ = json.Unmarshal([]byte(safeerr.JSONErrors(string(encoded))), &safe)
				message = safe["error"]
				if runes := []rune(message); len(runes) > 256 {
					message = string(runes[:256]) + "…"
				}
				if firstFailure == "" {
					firstFailure = message
				}
				if debug {
					s.log.Debug("bulk metadata refresh item failed", "item_id", id, "err", message)
				}
			}
		} else {
			refreshed++
		}
		complete(id)
	}
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

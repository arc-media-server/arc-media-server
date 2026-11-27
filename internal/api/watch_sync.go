package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"ams/internal/store"
)

// setResumePosition: PUT /api/items/{id}/resume {"position_ms": ...} sets a saved
// movie or episode position (0 clears it), with no player session or watch history changes.
func (s *Server) setResumePosition(w http.ResponseWriter, r *http.Request) {
	it, ok := s.syncPlayable(w, r)
	if !ok {
		return
	}
	var body struct {
		PositionMS *int64 `json:"position_ms"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || body.PositionMS == nil || *body.PositionMS < 0 ||
		decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "send a non-negative position_ms")
		return
	}
	if err := s.store.SetResumePosition(r.Context(), currentUserID(r), it.ID, *body.PositionMS); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type watchHistoryPage struct {
	Viewings   []store.WatchViewing `json:"viewings"`
	NextBefore int64                `json:"next_before,omitempty"`
}

// watchHistory: GET /api/watch-history lists the selected profile's completed
// viewings for visible items. Follow next_before with ?before= to page in descending
// viewing-ID order. Limit is 1–200 (default 50). Timestamps are Unix seconds.
func (s *Server) watchHistory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _, err := pageBounds(q.Get("limit"), "", 50, 200)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var before int64
	if raw := q.Get("before"); raw != "" {
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || before <= 0 {
			writeError(w, http.StatusBadRequest, "before must be a positive viewing id")
			return
		}
	}
	viewings, err := s.store.WatchViewings(r.Context(), currentUserID(r), access(r), before, limit+1)
	if err != nil {
		s.fail(w, err)
		return
	}
	page := watchHistoryPage{Viewings: viewings}
	if len(viewings) > limit {
		page.Viewings = viewings[:limit]
		page.NextBefore = viewings[limit-1].ID
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, page)
}

// addWatchViewing: POST /api/items/{id}/watch-history {"watched_at": ...} records a
// completed viewing at the supplied Unix timestamp. Retrying the same item and timestamp
// returns the existing viewing. This does not change watched status or resume position.
func (s *Server) addWatchViewing(w http.ResponseWriter, r *http.Request) {
	it, ok := s.syncPlayable(w, r)
	if !ok {
		return
	}
	var body struct {
		WatchedAt int64 `json:"watched_at"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || body.WatchedAt <= 0 || body.WatchedAt > time.Now().Unix() ||
		decoder.Decode(new(any)) != io.EOF {
		writeError(w, http.StatusBadRequest, "watched_at must be a positive Unix timestamp in seconds and not in the future")
		return
	}
	viewing, err := s.store.AddWatchViewing(r.Context(), currentUserID(r), it.ID, body.WatchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, viewing)
}

// removeWatchViewing: DELETE /api/items/{id}/watch-history/{viewing}?watched_at=...
// removes the matching viewing for this profile. Repeat deletes return 204. Watched
// status and resume position are independent of this operation.
func (s *Server) removeWatchViewing(w http.ResponseWriter, r *http.Request) {
	it, ok := s.syncPlayable(w, r)
	if !ok {
		return
	}
	id, idErr := strconv.ParseInt(r.PathValue("viewing"), 10, 64)
	at, atErr := strconv.ParseInt(r.URL.Query().Get("watched_at"), 10, 64)
	if idErr != nil || atErr != nil || id <= 0 || at <= 0 {
		writeError(w, http.StatusBadRequest, "viewing id and watched_at must be positive integers")
		return
	}
	viewing := store.WatchViewing{ID: id, ItemID: it.ID, WatchedAt: at}
	if err := s.store.RemoveWatchViewing(r.Context(), currentUserID(r), viewing); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) syncPlayable(w http.ResponseWriter, r *http.Request) (*store.Item, bool) {
	it, ok := s.itemFromPath(w, r)
	if ok && it.Kind != "movie" && it.Kind != "episode" {
		writeError(w, http.StatusBadRequest, "watch sync applies to movies and episodes")
		return nil, false
	}
	return it, ok
}

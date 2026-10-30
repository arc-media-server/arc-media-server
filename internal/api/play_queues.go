package api

import (
	"ams/internal/store"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type queueEntryDTO struct {
	ID        string   `json:"id"`
	Available bool     `json:"available"`
	ItemID    string   `json:"item_id,omitempty"`
	Item      *itemDTO `json:"item,omitempty"`
}
type playQueueDTO struct {
	ID             string          `json:"id"`
	Revision       int64           `json:"revision"`
	CurrentEntryID string          `json:"current_entry_id"`
	Shuffled       bool            `json:"shuffled"`
	ExpiresAt      int64           `json:"expires_at"`
	Entries        []queueEntryDTO `json:"entries"`
}

func (s *Server) queueFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrQueueNotFound):
		writeError(w, 404, "queue not found")
	case errors.Is(err, store.ErrQueueConflict):
		writeError(w, 409, err.Error())
	case errors.Is(err, store.ErrQueueInput):
		writeError(w, 400, "invalid queue operation, entries or limit (maximum 1000)")
	default:
		s.fail(w, err)
	}
}
func (s *Server) queueDTO(r *http.Request, q *store.PlayQueue) (playQueueDTO, error) {
	out := playQueueDTO{ID: q.ID, Revision: q.Revision, CurrentEntryID: q.CurrentEntryID, Shuffled: q.Shuffled, ExpiresAt: q.ExpiresAt, Entries: []queueEntryDTO{}}
	ids := make([]string, 0, len(q.Entries))
	for _, e := range q.Entries {
		ids = append(ids, e.ItemID)
	}
	items, err := s.store.QueueItems(r.Context(), ids, access(r))
	if err != nil {
		return out, err
	}
	var dtos []*itemDTO
	for _, e := range q.Entries {
		entry := queueEntryDTO{ID: e.ID}
		if it, ok := items[e.ItemID]; ok {
			dto := toDTO(it)
			entry.Available = true
			entry.ItemID = it.ID
			entry.Item = &dto
			dtos = append(dtos, &dto)
		}
		out.Entries = append(out.Entries, entry)
	}
	if err = s.fillUserData(r, dtos); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Server) queueItemsForRequest(r *http.Request, ids []string, playlistID string) ([]string, error) {
	if playlistID != "" {
		if len(ids) > 0 {
			return nil, store.ErrQueueInput
		}
		p, err := s.store.GetPlaylist(r.Context(), playlistID)
		if err != nil {
			return nil, err
		}
		if p == nil || p.UserID != currentUserID(r) {
			return nil, store.ErrQueueNotFound
		}
		_, items, err := s.playlistContents(r, p)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			ids = append(ids, it.ID)
		}
	} else {
		if len(ids) == 0 || len(ids) > store.MaxQueueEntries {
			return nil, store.ErrQueueInput
		}
		var expanded []string
		for _, id := range ids {
			it, err := s.store.GetItem(r.Context(), id)
			if err != nil {
				return nil, err
			}
			if it == nil {
				return nil, store.ErrQueueNotFound
			}
			visible, err := s.store.CanSee(r.Context(), it, access(r))
			if err != nil {
				return nil, err
			}
			if !visible {
				return nil, store.ErrQueueNotFound
			}
			if it.Kind == "series" {
				episodes, err := s.orderedEpisodeIDs(r, it)
				if err != nil {
					return nil, err
				}
				expanded = append(expanded, episodes...)
			} else if it.Kind == "movie" || it.Kind == "episode" {
				expanded = append(expanded, id)
			} else {
				return nil, store.ErrQueueInput
			}
			if len(expanded) > store.MaxQueueEntries {
				return nil, store.ErrQueueInput
			}
		}
		ids = expanded
	}
	if len(ids) == 0 || len(ids) > store.MaxQueueEntries {
		return nil, store.ErrQueueInput
	}
	visible, err := s.store.QueueItems(r.Context(), ids, access(r))
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, ok := visible[id]; !ok {
			return nil, store.ErrQueueNotFound
		}
	}
	return ids, nil
}

// createPlayQueue: POST /api/play-queues creates this device/profile's queue from item_ids
// (movies, episodes or expanded shows) or playlist_id. Playlist contents are snapshotted.
// Replacing an existing queue requires replace_id and revision. Up to 1000 entries including
// duplicates. Does not start/stop playback or change watched state.
func (s *Server) createPlayQueue(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ItemIDs    []string `json:"item_ids" api:"optional"`
		PlaylistID string   `json:"playlist_id" api:"optional"`
		ReplaceID  string   `json:"replace_id" api:"optional"`
		Revision   int64    `json:"revision" api:"optional"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeError(w, 400, "invalid queue request")
		return
	}
	ids, err := s.queueItemsForRequest(r, body.ItemIDs, body.PlaylistID)
	if err != nil {
		s.queueFailure(w, err)
		return
	}
	q, err := s.store.CreatePlayQueue(r.Context(), currentUserID(r), currentSession(r), ids, body.ReplaceID, body.Revision)
	if err != nil {
		s.queueFailure(w, err)
		return
	}
	s.log.Info("play queue created", "queue_id", q.ID, "entries", len(q.Entries))
	dto, err := s.queueDTO(r, q)
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 201, dto)
}

// getPlayQueue: GET /api/play-queues/{id} retrieves only this device/profile's queue. The id
// "current" recovers it after app restart. Queues expire after seven inactive days. Restricted
// or missing entries retain opaque entry IDs but expose no item IDs or metadata.
func (s *Server) getPlayQueue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "current" {
		id = ""
	}
	q, err := s.store.PlayQueue(r.Context(), currentUserID(r), currentSession(r), id)
	if err != nil {
		s.queueFailure(w, err)
		return
	}
	dto, err := s.queueDTO(r, q)
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, dto)
}

// mutatePlayQueue: PATCH /api/play-queues/{id} requires revision and operation: append/next
// with item_ids; remove/reorder with entry_ids; select with entry_id; clear, advance, shuffle or
// unshuffle. Reorder protects the current/past prefix; clear drops upcoming entries. Queue
// position changes never remotely control a player; clients start/stop playback separately.
func (s *Server) mutatePlayQueue(w http.ResponseWriter, r *http.Request) {
	var body store.QueueMutation
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || body.Revision <= 0 || len(body.EntryIDs) > store.MaxQueueEntries || len(body.ItemIDs) > store.MaxQueueEntries {
		writeError(w, 400, "invalid queue mutation")
		return
	}
	if _, err := s.store.PlayQueue(r.Context(), currentUserID(r), currentSession(r), r.PathValue("id")); err != nil {
		s.queueFailure(w, err)
		return
	}
	if body.Operation == "append" || body.Operation == "next" {
		ids, err := s.queueItemsForRequest(r, body.ItemIDs, "")
		if err != nil {
			s.queueFailure(w, err)
			return
		}
		body.ItemIDs = ids
	}
	q, err := s.store.MutatePlayQueue(r.Context(), currentUserID(r), currentSession(r), r.PathValue("id"), body)
	if err != nil {
		s.queueFailure(w, err)
		return
	}
	dto, err := s.queueDTO(r, q)
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, dto)
}

// deletePlayQueue: DELETE /api/play-queues/{id}?revision=<revision> deletes this device/profile's
// queue. The client stops its own player separately. Session revocation also deletes queues.
func (s *Server) deletePlayQueue(w http.ResponseWriter, r *http.Request) {
	revision, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
	if err != nil || revision <= 0 {
		writeError(w, 400, "send the queue revision")
		return
	}
	if err = s.store.DeletePlayQueue(r.Context(), currentUserID(r), currentSession(r), r.PathValue("id"), revision); err != nil {
		s.queueFailure(w, err)
		return
	}
	w.WriteHeader(204)
}

package api

import (
	"ams/internal/store"
	"net/http"
)

// itemData is owned by one request/profile. Repeated cards across rows
// reuse watch state and episode counts without keeping stale or cross-profile server caches.
type itemData struct {
	server  *Server
	request *http.Request
	known   map[string]bool
	users   map[string]store.UserData
	counted map[string]bool
	counts  map[string]int
}

func (s *Server) itemData(r *http.Request) *itemData {
	return &itemData{s, r, map[string]bool{}, map[string]store.UserData{}, map[string]bool{}, map[string]int{}}
}

func (d *itemData) fill(items []*itemDTO) error {
	user := currentUserID(d.request)
	if user == "" || len(items) == 0 {
		return nil
	}
	ids, series := []string{}, []string{}
	pending, counting := map[string]bool{}, map[string]bool{}
	for _, it := range items {
		if !d.known[it.ID] && !pending[it.ID] {
			ids = append(ids, it.ID)
			pending[it.ID] = true
		}
		if it.Kind == "series" && !d.counted[it.ID] && !counting[it.ID] {
			series = append(series, it.ID)
			counting[it.ID] = true
		}
	}
	if len(ids) > 0 {
		values, err := d.server.store.UserDataFor(d.request.Context(), user, ids)
		if err != nil {
			return err
		}
		for _, id := range ids {
			d.users[id] = values[id]
			d.known[id] = true
		}
	}
	if len(series) > 0 {
		values, err := d.server.store.UnwatchedCounts(d.request.Context(), user, series)
		if err != nil {
			return err
		}
		for _, id := range series {
			d.counts[id] = values[id]
			d.counted[id] = true
		}
	}
	for _, it := range items {
		value := d.users[it.ID]
		it.UserData = &value
		if it.Kind == "series" {
			count := d.counts[it.ID]
			it.UnwatchedCount = &count
		}
	}
	return nil
}

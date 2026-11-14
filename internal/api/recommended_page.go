package api

import "net/http"

type recommendedItemsPage struct {
	Items   []itemDTO `json:"items"`
	Total   int       `json:"total"`
	Offset  int       `json:"offset"`
	HasMore bool      `json:"has_more"`
	// True when a personalized candidate pool hit its existing bounded selection window.
	Bounded bool `json:"bounded"`
}

// recommendedPage: GET /api/libraries/{id}/recommended/rows/{row}?limit=50&offset=20
// continues a current visible row using its opaque id. Limit 1–100, offset 0–10000000.
// Missing/hidden/replaced daily rows return 404; refetch recommended to discover current IDs.
// Ordinary rows page their full filtered library; curated rows page at most 500 candidates
// (Continue Watching uses the existing 30 in-progress / 40 series windows). No provider calls.
func (s *Server) recommendedPage(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	limit, offset, err := pageBounds(r.URL.Query().Get("limit"), r.URL.Query().Get("offset"), 50, 100)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	layout, err := s.readRecommendedLayout(r, lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	rows, err := s.recommendedRowsWithLayout(r, lib, recommendedLimit, r.PathValue("row"), layout)
	if err != nil {
		s.fail(w, err)
		return
	}
	for _, row := range arrangeRecommended(rows, layout) {
		if row.ID != r.PathValue("row") {
			continue
		}
		page := recommendedItemsPage{Items: []itemDTO{}, Offset: offset}
		if row.query != nil {
			q := *row.query
			q.Limit, q.Offset = limit, offset
			items, total, e := s.store.ListItems(r.Context(), q)
			if e != nil {
				s.fail(w, e)
				return
			}
			page.Items, page.Total = toDTOs(items), total
			if q.Kind == "episode" {
				entries := make([]homeEntry, len(items))
				for i, it := range items {
					entries[i] = homeEntry{item: it}
				}
				page.Items, err = s.homeDTOs(r, entries)
			} else {
				err = s.fillUserData(r, ptrs(page.Items))
			}
			if err != nil {
				s.fail(w, err)
				return
			}
		} else {
			expanded, e := s.recommendedRowsLimitTarget(r, lib, 500, row.ID)
			if e != nil {
				s.fail(w, e)
				return
			}
			for _, full := range expanded {
				if full.ID == row.ID {
					page.Total = len(full.Items)
					page.Bounded = true
					if offset < page.Total {
						page.Items = full.Items[offset:min(page.Total, offset+limit)]
					}
					break
				}
			}
		}
		page.HasMore = offset+len(page.Items) < page.Total
		w.Header().Set("Cache-Control", "private, no-store")
		writeJSON(w, 200, page)
		return
	}
	writeError(w, 404, "recommendation row no longer available")
}

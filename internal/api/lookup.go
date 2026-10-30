package api

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var imdbLookupID = regexp.MustCompile(`^tt[0-9]{1,16}$`)

type itemBatch struct {
	Items []itemDTO `json:"items"`
}

// lookupItems: GET /api/items/lookup?provider=tmdb|tvdb|imdb&provider_id=... returns all matching
// accessible identities. Optional kind=movie|series|episode, library_id, limit (1–100, default 50)
// and offset (0 or greater). Missing and restricted matches both produce an empty page. No provider
// request, scan or probe is started, and existing offline identities may be returned.
func (s *Server) lookupItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	provider := q.Get("provider")
	id := q.Get("provider_id")
	kind := q.Get("kind")
	if provider == "imdb" {
		id = strings.ToLower(id)
		if !imdbLookupID.MatchString(id) {
			writeError(w, 400, "provider_id must be an IMDb tt identifier")
			return
		}
	} else if provider == "tmdb" || provider == "tvdb" {
		n, err := strconv.ParseInt(id, 10, 64)
		if err != nil || n <= 0 {
			writeError(w, 400, "provider_id must be a positive integer")
			return
		}
		id = strconv.FormatInt(n, 10)
	} else {
		writeError(w, 400, "provider must be tmdb, tvdb or imdb")
		return
	}
	if kind != "" && kind != "movie" && kind != "series" && kind != "episode" {
		writeError(w, 400, "invalid kind")
		return
	}
	limit, offset, err := pageBounds(q.Get("limit"), q.Get("offset"), 50, 100)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	var library int64
	if raw := q.Get("library_id"); raw != "" {
		library, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || library <= 0 {
			writeError(w, 400, "library_id must be positive")
			return
		}
	}
	items, total, err := s.store.LookupItems(r.Context(), provider, id, kind, library, limit, offset, access(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	out := toDTOs(items)
	if out == nil {
		out = []itemDTO{}
	}
	if err = s.fillUserData(r, ptrs(out)); err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, itemPage{Items: out, Total: total, Offset: offset})
}

// batchItems: POST /api/items/batch {"ids":[...]} retrieves up to 100 item details in requested
// order (duplicates retained). Unknown/restricted IDs are omitted identically, without missing
// counts. Same metadata/files/watch fields as individual item GET. This is read-only despite POST.
func (s *Server) batchItems(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || len(body.IDs) == 0 || len(body.IDs) > 100 {
		writeError(w, 400, "send between 1 and 100 ids")
		return
	}
	if decoder.Decode(new(any)) != io.EOF {
		writeError(w, 400, "send one JSON object")
		return
	}
	for _, id := range body.IDs {
		if len(id) != 32 || strings.IndexFunc(id, func(r rune) bool { return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') }) >= 0 {
			writeError(w, 400, "ids must be Arc item IDs")
			return
		}
	}
	found, err := s.store.VisibleItemsByID(r.Context(), body.IDs, access(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	out := itemBatch{Items: []itemDTO{}}
	cache := map[string]itemDTO{}
	for _, id := range body.IDs {
		it, ok := found[id]
		if !ok {
			continue
		}
		dto, ok := cache[id]
		if !ok {
			dto, err = s.itemDetails(r, &it)
			if err != nil {
				s.fail(w, err)
				return
			}
			cache[id] = dto
		}
		out.Items = append(out.Items, dto)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, out)
}

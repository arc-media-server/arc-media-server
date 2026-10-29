package api

import (
	"encoding/json"
	"net/http"
	"slices"
)

type recommendedLook struct {
	Type   string `json:"type"`
	Hidden bool   `json:"hidden,omitempty"`
	Style  string `json:"style,omitempty"`
}

var recommendedTypes = []string{"continue", "recently_released", "recently_added", "start_watching", "because_you_watched", "top_rated", "recently_watched", "rediscover", "more_in_genre", "top_in_genre", "top_by_person", "top_unwatched"}

func (s *Server) readRecommendedLayout(r *http.Request, id int64) ([]recommendedLook, error) {
	raw, err := s.store.RecommendedLayout(r.Context(), id)
	if err != nil {
		return nil, err
	}
	rows := []recommendedLook{}
	err = json.Unmarshal([]byte(raw), &rows)
	return rows, err
}

// getRecommendedLayout: GET /api/libraries/{id}/recommended/layout returns the library's shared row-family order, visibility and style (admin).
func (s *Server) getRecommendedLayout(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	rows, err := s.readRecommendedLayout(r, lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

// setRecommendedLayout: PUT /api/libraries/{id}/recommended/layout {"rows":[{"type","hidden","style"}]} sets shared Recommended rows (admin). Empty rows resets defaults; omitted/new families remain visible. Styles: poster, thumb, arc or empty for automatic.
func (s *Server) setRecommendedLayout(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	if lib.Kind == "videos" {
		writeError(w, 400, "Other videos libraries have no Recommended view")
		return
	}
	var body struct {
		Rows []recommendedLook `json:"rows"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || body.Rows == nil || len(body.Rows) > len(recommendedTypes) {
		writeError(w, 400, `send {"rows": [row families]}`)
		return
	}
	seen := map[string]bool{}
	for _, row := range body.Rows {
		if !slices.Contains(recommendedTypes, row.Type) || seen[row.Type] || !slices.Contains([]string{"", "poster", "thumb", "arc"}, row.Style) {
			writeError(w, 400, "unknown or duplicate row family, or invalid style")
			return
		}
		seen[row.Type] = true
	}
	raw, _ := json.Marshal(body.Rows)
	if err := s.store.SetRecommendedLayout(r.Context(), lib.ID, string(raw)); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("library recommended layout updated", "library", lib.Name, "families", len(body.Rows))
	writeJSON(w, http.StatusOK, body)
}

func arrangeRecommended(rows []recommendedRow, layout []recommendedLook) []recommendedRow {
	out := []recommendedRow{}
	used := map[string]bool{}
	for _, look := range layout {
		if used[look.Type] {
			continue
		}
		used[look.Type] = true
		for _, row := range rows {
			if row.Type == look.Type && !look.Hidden {
				row.Style = look.Style
				out = append(out, row)
			}
		}
	}
	for _, row := range rows {
		if !used[row.Type] {
			out = append(out, row)
		}
	}
	return out
}

package api

import (
	"net/http"
	"strconv"

	"ams/internal/store"
)

// libraryHealth: GET /api/libraries/{id}/health reports everything in a library worth a look:
// unidentified items, unreadable, unrecognised and missing files, duplicates and missing episodes.
func (s *Server) libraryHealth(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	unidentified, err := s.store.Unmatched(ctx, lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	unreadable, err := s.store.UnreadableFiles(ctx, lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	unrecognised, err := s.store.UnrecognisedFiles(ctx, lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	missing, err := s.store.MissingFiles(ctx, lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	dups, err := s.store.Duplicates(ctx, lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	var gaps []store.SeriesGaps
	if lib.Kind == "shows" {
		if gaps, err = s.store.MissingEpisodes(ctx, lib); err != nil {
			s.fail(w, err)
			return
		}
	}

	type duplicateDTO struct {
		Kind  string              `json:"kind"`
		Items []itemDTO           `json:"items"`
		Files []store.FileProblem `json:"files,omitempty"`
	}
	dupOut := make([]duplicateDTO, 0, len(dups))
	for _, d := range dups {
		dupOut = append(dupOut, duplicateDTO{Kind: d.Kind, Items: toDTOs(d.Items), Files: d.Files})
	}
	type gapsDTO struct {
		Series   itemDTO                `json:"series"`
		Ordering string                 `json:"ordering"`
		Have     int                    `json:"have"`
		Unplaced int                    `json:"unplaced"`
		Missing  []store.MissingEpisode `json:"missing"`
	}
	gapsOut := make([]gapsDTO, 0, len(gaps))
	for _, g := range gaps {
		gapsOut = append(gapsOut, gapsDTO{Series: toDTO(g.Series), Ordering: g.Ordering, Have: g.Have,
			Unplaced: g.Unplaced, Missing: g.Missing})
	}

	// Unidentified episodes are listed by themselves, so say which series each is from.
	unidentifiedOut := toDTOs(unidentified)
	for i := range unidentifiedOut {
		if d := &unidentifiedOut[i]; d.Kind == "episode" {
			if p, err := s.store.GetItem(ctx, d.ParentID); err == nil && p != nil {
				d.SeriesTitle = p.Title
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"library":          lib,
		"unidentified":     unidentifiedOut,
		"unreadable":       unreadable,
		"unrecognised":     unrecognised,
		"missing_files":    missing,
		"duplicates":       dupOut,
		"missing_episodes": gapsOut,
	})
}

// healthSummary: GET /api/health/libraries counts, per library id, the problems that need the
// user (unidentified items, unreadable and unrecognised files).
func (s *Server) healthSummary(w http.ResponseWriter, r *http.Request) {
	counts, err := s.store.ProblemCounts(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	out := map[string]int{}
	for lib, n := range counts {
		out[strconv.FormatInt(lib, 10)] = n
	}
	writeJSON(w, http.StatusOK, out)
}

func toDTOs(items []store.Item) []itemDTO {
	out := make([]itemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, toDTO(it))
	}
	return out
}

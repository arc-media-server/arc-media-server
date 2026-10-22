package api

import (
	"net/http"
	"strconv"

	"ams/internal/backup"
	"ams/internal/store"
)

// libraryHealth: GET /api/libraries/{id}/health reports everything in a library worth a look:
// unidentified items, unreadable, unrecognised and missing files, duplicates and missing episodes.
// watch_problem describes failed folder notifications and the configured scheduled fallback.
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

	watchProblem := ""
	if s.watchProblem != nil {
		watchProblem = s.watchProblem(lib.ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"watch_problem":    watchProblem,
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
	if s.watchProblem != nil {
		libs, err := s.store.ListLibraries(r.Context())
		if err != nil {
			s.fail(w, err)
			return
		}
		for _, lib := range libs {
			if s.watchProblem(lib.ID) != "" {
				out[strconv.FormatInt(lib.ID, 10)]++
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// emptyTrash: POST /api/libraries/{id}/empty-trash forgets everything in a library whose files are
// gone (after backing up the database). Media files aren't touched.
func (s *Server) emptyTrash(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	if s.scanner.Status(lib.ID).Running {
		writeError(w, http.StatusConflict, "the library is being scanned; try again when it finishes")
		return
	}
	if _, err := s.backups.Create(r.Context(), backup.ReasonBeforeDelete); err != nil {
		writeError(w, http.StatusInternalServerError, "couldn't back up first, so nothing was removed: "+err.Error())
		return
	}
	n, err := s.store.EmptyTrash(r.Context(), lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("removed titles whose files are gone", "library", lib.Name, "count", n)
	writeJSON(w, http.StatusOK, map[string]int64{"removed": n})
}

// removeItem: DELETE /api/items/{id} forgets a movie, episode or show whose files are all gone
// (after backing up the database). Refused if any of its files is still there.
func (s *Server) removeItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	if _, err := s.backups.Create(r.Context(), backup.ReasonBeforeDelete); err != nil {
		writeError(w, http.StatusInternalServerError, "couldn't back up first, so nothing was removed: "+err.Error())
		return
	}
	removed, err := s.store.RemoveGoneItem(r.Context(), it)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !removed {
		writeError(w, http.StatusConflict, "some of its files are still there; only titles whose files are all gone can be removed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toDTOs(items []store.Item) []itemDTO {
	out := make([]itemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, toDTO(it))
	}
	return out
}

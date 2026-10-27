package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"ams/internal/schedule"
)

// startLibraryWork: POST /api/libraries/{id}/work (admin) starts {"operation":"refresh"|"analyze"}.
// Refresh refetches known titles regardless of age; analyze rereads known present files, not folders.
// Both preserve locked metadata/history and share the library's scan/maintenance lease.
func (s *Server) startLibraryWork(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Operation string `json:"operation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Operation != "refresh" && body.Operation != "analyze") {
		writeError(w, 400, "operation is refresh or analyze")
		return
	}
	if body.Operation == "refresh" && lib.Kind == "videos" {
		writeError(w, 400, "home videos have no online metadata to refresh")
		return
	}
	if !s.scanner.StartOperation(lib.ID, body.Operation) {
		writeError(w, 409, "work is already running for this library")
		return
	}
	writeJSON(w, 202, s.scanner.Status(lib.ID))
}

// analyzeItem: POST /api/items/{id}/analyze (admin) rereads a movie's or episode's files, or every
// episode of a show, like Plex's Analyze. It runs in the background under the library's lease
// (stoppable from Activity) and keeps watch history, markers and edits.
func (s *Server) analyzeItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	files, err := s.store.ItemAnalysisFiles(r.Context(), it.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if len(files) == 0 {
		writeError(w, 409, "none of its files are present to analyze")
		return
	}
	if !s.scanner.StartItemAnalysis(it.LibraryID, it.ID) {
		writeError(w, 409, "the library is busy; try again when its current work finishes")
		return
	}
	writeJSON(w, 202, s.scanner.Status(it.LibraryID))
}

// stopLibraryWork: POST /api/libraries/{id}/stop (admin) stops {"job_id":...} only.
// It cancels queued follow-ups, never reconciles unvisited media, and leaves future triggers enabled.
func (s *Server) stopLibraryWork(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		JobID int64 `json:"job_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.JobID <= 0 {
		writeError(w, 400, "send the current job_id")
		return
	}
	if !s.scanner.Stop(lib.ID, body.JobID) {
		writeError(w, 409, "that run is no longer active; refresh its status")
		return
	}
	writeJSON(w, 202, s.scanner.Status(lib.ID))
}

// stopActivity: POST /api/activity/{id}/stop (admin) requests a stop for that cancellable task.
func (s *Server) stopActivity(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 400, "invalid task id")
		return
	}
	if !s.activity.Stop(id) {
		writeError(w, 409, "that task cannot be stopped or has finished")
		return
	}
	writeJSON(w, 202, map[string]bool{"stopping": true})
}

// maintenanceWindow: GET /api/maintenance/window (admin) reports the actual server clock,
// timezone/offset and current or next maintenance window, including overnight and all-day policy.
func (s *Server) maintenanceWindow(w http.ResponseWriter, r *http.Request) {
	settings := s.settings()
	writeJSON(w, 200, schedule.New(settings.MaintenanceStart, settings.MaintenanceEnd).Info(time.Now()))
}

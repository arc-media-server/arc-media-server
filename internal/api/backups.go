package api

import (
	"context"
	"errors"
	"net/http"
	"os"

	"ams/internal/backup"
)

// listBackups: GET /api/backups. Database backups, newest first.
func (s *Server) listBackups(w http.ResponseWriter, r *http.Request) {
	list, err := s.backups.List()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"folder": s.backups.Dir(), "backups": list})
}

// createBackup: POST /api/backups takes a manual backup now.
func (s *Server) createBackup(w http.ResponseWriter, r *http.Request) {
	b, err := s.backups.Create(r.Context(), backup.ReasonManual)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

// restoreBackup: POST /api/backups/{name}/restore replaces the database with a backup. The current
// state is saved as a backup first. Refused while a scan is running.
func (s *Server) restoreBackup(w http.ResponseWriter, r *http.Request) {
	release, ok := s.beginCatalogMutation(w, r)
	if !ok {
		return
	}
	defer release()
	if err := s.backups.Restore(r.Context(), r.PathValue("name")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "backup not found")
			return
		}
		s.log.Warn("restore failed", "backup", r.PathValue("name"), "err", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.afterRestore != nil {
		if err := s.afterRestore(context.WithoutCancel(r.Context())); err != nil {
			s.log.Error("database restored but runtime reload failed", "err", err)
			writeError(w, http.StatusInternalServerError, "The database was restored, but Arc couldn't apply all restored settings. Restart Arc. "+err.Error())
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// deleteBackup: DELETE /api/backups/{name}. Deletes a backup.
func (s *Server) deleteBackup(w http.ResponseWriter, r *http.Request) {
	if err := s.backups.Delete(r.PathValue("name")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "backup not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

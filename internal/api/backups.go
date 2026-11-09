package api

import (
	"context"
	"errors"
	"net/http"
	"os"

	"ams/internal/backup"
	"ams/internal/store"
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
	if s.scanner.Running() {
		writeError(w, http.StatusConflict, "a library scan is running; try again when it finishes")
		return
	}
	if err := s.backups.Restore(r.Context(), r.PathValue("name")); err != nil {
		if errors.Is(err, backup.ErrNotFound) {
			writeError(w, http.StatusNotFound, "backup not found")
			return
		}
		if errors.Is(err, backup.ErrInvalidName) || errors.Is(err, store.ErrInvalidBackup) {
			err = errMsg(err.Error())
		}
		s.inputError(w, r, err)
		return
	}
	if s.afterRestore != nil {
		if err := s.afterRestore(context.WithoutCancel(r.Context())); err != nil {
			markFailureLogged(w)
			s.log.Error("database restored but runtime reload failed", "err", err, "request_id", w.Header().Get(requestIDHeader))
			writeError(w, http.StatusInternalServerError, "The database was restored, but Arc couldn't apply all restored settings. Restart Arc.")
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
		if errors.Is(err, backup.ErrInvalidName) {
			err = errMsg(err.Error())
		}
		s.inputError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

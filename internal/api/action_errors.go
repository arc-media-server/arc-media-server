package api

import (
	"errors"
	"net/http"
)

// errMsg is a deliberate, safe client-validation message. Unexpected dependency errors must
// not become a 400 merely because the operation started with user input.
type errMsg string

func (e errMsg) Error() string { return string(e) }

func (s *Server) inputError(w http.ResponseWriter, r *http.Request, err error) {
	if r.Context().Err() != nil {
		return
	}
	var invalid errMsg
	if errors.As(err, &invalid) {
		writeError(w, http.StatusBadRequest, invalid.Error())
		return
	}
	s.fail(w, err)
}

package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"ams/internal/auth"
)

// Friends: people outside the household, shared some libraries as in Plex. The admin makes one
// (Profiles > Add Friend) and sends them the invite link; they choose a password with it, then
// sign in with their name and password from anywhere. They never see the household's profiles.

// inviteLife is how long an invite link works.
const inviteLife = 7 * 24 * time.Hour

// A public, non-credential hash at auth.Hash's default bcrypt cost. Compare even when a name
// does not exist or an invite has no password yet. Its result can never authorize a profile.
const missingFriendPasswordHash = "$2a$10$1sDNByczecGpB1U8HAteg.c2BDFeDOGkvNhUyD.j0FLWQ1iWEAWKK"

// newInvite makes a friend's invite link token (a new one replaces the old).
func (s *Server) newInvite(r *http.Request, userID string) (string, error) {
	token, hash := auth.NewToken()
	return token, s.store.SetInvite(r.Context(), userID, hash, time.Now().Add(inviteLife).Unix())
}

// reinviteFriend: POST /api/profiles/{id}/invite (admin) makes a new invite link for a friend
// (e.g. one who's lost theirs, or to reset their password).
func (s *Server) reinviteFriend(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if u == nil || !u.IsFriend {
		writeError(w, http.StatusNotFound, "no such friend")
		return
	}
	token, err := s.newInvite(r, u.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"invite_token": token})
}

// getInvite: GET /api/invites/{token} says who an invite link is for (public).
func (s *Server) getInvite(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.UserByInvite(r.Context(), auth.HashToken(r.PathValue("token")))
	if err != nil {
		s.fail(w, err)
		return
	}
	if u == nil {
		writeError(w, http.StatusNotFound, "this invite link has run out or been used; ask for a new one")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": u.Name})
}

// acceptInvite: POST /api/invites/{token} {"password"} sets the friend's password and signs them in.
func (s *Server) acceptInvite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	key := "invite:" + clientIP(r)
	if !s.limiter.Attempt(key) {
		writeError(w, http.StatusTooManyRequests, auth.ErrTooManyAttempts.Error())
		return
	}
	ctx := r.Context()
	u, err := s.store.UserByInvite(ctx, auth.HashToken(r.PathValue("token")))
	if err != nil {
		s.fail(w, err)
		return
	}
	if u == nil {
		writeError(w, http.StatusNotFound, "this invite link has run out or been used; ask for a new one")
		return
	}
	if len(body.Password) < auth.MinPasswordLength {
		writeError(w, http.StatusBadRequest, "the password needs at least 8 characters")
		return
	}
	hash, err := auth.Hash(body.Password)
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.store.AcceptInvite(ctx, u.ID, hash); err != nil {
		s.fail(w, err)
		return
	}
	// A new password signs them out everywhere else.
	if err := s.store.DeleteUserSessions(ctx, u.ID); err != nil {
		s.fail(w, err)
		return
	}
	s.limiter.Succeed(key)
	if _, err := s.startSession(w, r, u.ID, "friend"); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("friend joined", "name", u.Name)
	w.WriteHeader(http.StatusNoContent)
}

// friendLogin: POST /api/auth/friend {"name", "password"} signs a friend in.
func (s *Server) friendLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		// An app signing in (see login).
		Device string `json:"device"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	key := "friend:" + clientIP(r)
	if !s.limiter.Attempt(key) {
		writeError(w, http.StatusTooManyRequests, auth.ErrTooManyAttempts.Error())
		return
	}
	u, err := s.store.FriendByName(r.Context(), body.Name)
	if err != nil {
		s.fail(w, err)
		return
	}
	hash := missingFriendPasswordHash
	if u != nil && u.PasswordHash != "" {
		hash = u.PasswordHash
	}
	valid := auth.Check(hash, body.Password)
	if u == nil || u.PasswordHash == "" || !valid {
		s.log.Warn("friend sign-in failed", "name", body.Name, "address", clientIP(r))
		writeError(w, http.StatusUnauthorized, "that name and password don't match")
		return
	}
	s.limiter.Succeed(key)
	s.log.Info("friend signed in", "name", u.Name, "address", clientIP(r), "device", deviceName(r.UserAgent()))
	old, ok := s.readSession(w, r)
	if !ok {
		return
	}
	if old != nil && body.Device == "" {
		s.store.DeleteSession(r.Context(), old.TokenHash)
	}
	s.signIn(w, r, u.ID, "friend", strings.TrimSpace(body.Device))
}

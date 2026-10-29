package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"ams/internal/auth"
	"ams/internal/remote"
	"ams/internal/settings"
	"ams/internal/store"
)

const sessionCookie = "ams_session"

var regionRe = regexp.MustCompile(`^[A-Z]{2}$`)

// Profile colours a profile can have (matched by the web app).
var profileColors = map[string]bool{"violet": true, "blue": true, "teal": true, "green": true, "amber": true,
	"orange": true, "rose": true, "slate": true}

type ctxKey int

const (
	userKey ctxKey = iota
	sessionKey
)

// currentSession identifies the signed-in browser or device (its session's token hash).
func currentSession(r *http.Request) string {
	h, _ := r.Context().Value(sessionKey).(string)
	return h
}

// currentUser is the profile making the request (set by signedIn).
func currentUser(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

// currentUserID is the profile's id, or "" (the server default) when there's none.
func currentUserID(r *http.Request) string {
	if u := currentUser(r); u != nil {
		return u.ID
	}
	return ""
}

// session finds the request's valid session, if any. A session started without the password only
// counts while the request still comes from the home network and that's still allowed.
func (s *Server) session(r *http.Request) *store.Session {
	token := requestToken(r)
	if token == "" {
		return nil
	}
	se, err := s.store.GetSession(r.Context(), auth.HashToken(token))
	if err != nil || se == nil {
		return nil
	}
	if se.Via == "local" && !s.bypassApplies(r) {
		return nil
	}
	if time.Since(time.Unix(se.LastSeen, 0)) > 5*time.Minute {
		s.store.RecordSessionUse(se.TokenHash)
	}
	return se
}

// requestToken is the session token a request carries: an app's "Authorization: Bearer" header,
// or the browser's cookie.
func requestToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value
	}
	return ""
}

// bypassApplies reports whether this request may pick a profile without the password.
func (s *Server) bypassApplies(r *http.Request) bool {
	return s.settings().LocalBypass && auth.IsLocal(r)
}

// signedIn wraps handlers that need a profile.
func (s *Server) signedIn(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		se := s.session(r)
		if se == nil || se.UserID == "" {
			writeError(w, http.StatusUnauthorized, "sign in and choose a profile")
			return
		}
		u, err := s.store.GetUser(r.Context(), se.UserID)
		if err != nil {
			s.fail(w, err)
			return
		}
		if u == nil {
			writeError(w, http.StatusUnauthorized, "choose a profile")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, u)
		requestStage(r, "handling request", 0)
		h(w, r.WithContext(context.WithValue(ctx, sessionKey, se.TokenHash)))
	}
}

// adminOnly wraps handlers only the admin profile may use.
func (s *Server) adminOnly(h http.HandlerFunc) http.HandlerFunc {
	return s.signedIn(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsAdmin {
			writeError(w, http.StatusForbidden, "only the admin profile can do this")
			return
		}
		h(w, r)
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true,
		Secure: secureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: 365 * 24 * 3600})
}

// A proxy's HTTPS hint may only strengthen cookie protection. It never grants local access,
// and a forged "http" header cannot weaken a connection that Arc serves over TLS itself.
func secureCookie(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}

// signIn starts a session: for an app (device named), it answers with the token for its
// Authorization header; for a browser, it sets the cookie and answers 204.
func (s *Server) signIn(w http.ResponseWriter, r *http.Request, userID, via, device string) {
	if device == "" {
		if _, err := s.startSession(w, r, userID, via); err != nil {
			s.fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	token, err := s.deviceSession(r, userID, via, device)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// deviceSession creates an app's session and returns its token.
func (s *Server) deviceSession(r *http.Request, userID, via, device string) (string, error) {
	token, hash := auth.NewToken()
	if err := s.store.CreateSession(r.Context(), hash, userID, via, r.UserAgent()); err != nil {
		return "", err
	}
	if len(device) > 80 {
		device = device[:80]
	}
	return token, s.store.SetSessionDevice(r.Context(), hash, device)
}

// startSession creates a session and sets its cookie.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID, via string) (string, error) {
	token, hash := auth.NewToken()
	if err := s.store.CreateSession(r.Context(), hash, userID, via, r.UserAgent()); err != nil {
		return "", err
	}
	s.setSessionCookie(w, r, token)
	return hash, nil
}

type profileDTO struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Color   string `json:"color"`
	HasPIN  bool   `json:"has_pin"`
	IsAdmin bool   `json:"is_admin"`
	// Age limit: the oldest age rating the profile may watch, 0 for none.
	MaxAge int `json:"max_age"`
	// The libraries the profile may see; null for all of them.
	LibraryIDs []int64 `json:"library_ids"`
	// Titles with these labels are hidden from the profile.
	ExcludeLabels []string `json:"exclude_labels"`
	// A friend, and whether they've accepted their invite (set a password) yet.
	IsFriend bool `json:"is_friend,omitempty"`
	Joined   bool `json:"joined,omitempty"`
	// A new friend's invite link token (only when it's just been made).
	InviteToken string `json:"invite_token,omitempty"`
}

func toProfile(u store.User) profileDTO {
	return profileDTO{ID: u.ID, Name: u.Name, Color: u.Color, HasPIN: u.HasPIN, IsAdmin: u.IsAdmin, MaxAge: u.MaxAge,
		LibraryIDs: u.LibraryIDs, ExcludeLabels: u.ExcludeLabels, IsFriend: u.IsFriend, Joined: u.IsFriend && u.PasswordHash != ""}
}

// authStatus: GET /api/auth/status tells the web app what to show: first-run setup, the sign-in
// screen, the profile picker, or the app.
func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	n, err := s.store.CountUsers(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := map[string]any{"setup_needed": n == 0, "signed_in": false, "profile": nil, "profiles": []profileDTO{},
		"password_session": false, "local": auth.IsLocal(r), "bypass": s.bypassApplies(r)}
	se := s.session(r)
	if se != nil {
		out["signed_in"] = true
		out["password_session"] = se.Via == "password"
		if se.UserID != "" {
			if u, err := s.store.GetUser(ctx, se.UserID); err == nil && u != nil {
				out["profile"] = toProfile(*u)
			}
		}
	}
	if (se != nil || s.bypassApplies(r)) && (se == nil || se.Via != "friend") {
		users, err := s.store.ListUsers(ctx)
		if err != nil {
			s.fail(w, err)
			return
		}
		profiles := make([]profileDTO, 0, len(users))
		for _, u := range users {
			if !u.IsFriend { // friends sign in with their own name and password
				profiles = append(profiles, toProfile(u))
			}
		}
		out["profiles"] = profiles
	}
	out["friend"] = se != nil && se.Via == "friend"
	writeJSON(w, http.StatusOK, out)
}

// setup: POST /api/auth/setup {"name", "password", "local_bypass", "tmdb_api_key", "tvdb_api_key"}
// creates the admin profile the first time AMS runs, and signs in as it.
func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var body struct {
		Name        string `json:"name"`
		Password    string `json:"password"`
		LocalBypass bool   `json:"local_bypass"`
		TMDBKey     string `json:"tmdb_api_key"`
		TVDBKey     string `json:"tvdb_api_key"`
		Region      string `json:"region"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	s.setupMu.Lock()
	defer s.setupMu.Unlock()
	if n, err := s.store.CountUsers(ctx); err != nil {
		s.fail(w, err)
		return
	} else if n > 0 {
		writeError(w, http.StatusConflict, "Arc is already set up")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "enter your name")
		return
	}
	if len(body.Password) < auth.MinPasswordLength {
		writeError(w, http.StatusBadRequest, "the password needs at least 8 characters")
		return
	}
	next := s.settings()
	next.LocalBypass = body.LocalBypass
	if k := strings.TrimSpace(body.TMDBKey); k != "" {
		next.TMDBKey = k
	}
	if k := strings.TrimSpace(body.TVDBKey); k != "" {
		next.TVDBKey = k
	}
	if regionRe.MatchString(body.Region) {
		next.Region = body.Region
	}
	if err := s.applySettings(ctx, next); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.Hash(body.Password)
	if err != nil {
		s.fail(w, err)
		return
	}
	u, err := s.store.CreateUser(ctx, body.Name, true, "violet", hash, "")
	if err != nil {
		s.fail(w, err)
		return
	}
	if _, err := s.startSession(w, r, u.ID, "password"); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("Arc set up", "admin", u.Name)
	writeJSON(w, http.StatusCreated, toProfile(*u))
}

// login: POST /api/auth/login {"password"} signs in with the household password. A profile is
// picked next.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
		// An app signing in: its name, e.g. "Living Room Apple TV". It gets a token back.
		Device string `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	key := "login:" + clientIP(r)
	if !s.limiter.Allow(key) {
		s.log.Warn("sign-in blocked after too many attempts", "address", clientIP(r))
		writeError(w, http.StatusTooManyRequests, auth.ErrTooManyAttempts.Error())
		return
	}
	admin, err := s.store.AdminUser(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	if admin == nil || !auth.Check(admin.PasswordHash, body.Password) {
		s.limiter.Fail(key)
		s.log.Warn("sign-in failed: wrong password", "address", clientIP(r), "device", deviceName(r.UserAgent()))
		writeError(w, http.StatusUnauthorized, "that password isn't right")
		return
	}
	s.limiter.Succeed(key)
	s.log.Info("signed in with the password", "address", clientIP(r), "device", deviceName(r.UserAgent()))
	if old := s.session(r); old != nil && body.Device == "" {
		s.store.DeleteSession(r.Context(), old.TokenHash)
	}
	s.signIn(w, r, "", "password", strings.TrimSpace(body.Device))
}

// chooseProfile: POST /api/auth/profile {"id", "pin", "password"} switches to a profile. A PIN is
// needed if the profile has one. On the home network without the password, the admin profile needs
// its PIN or the password, so no one else in the house can change settings.
func (s *Server) chooseProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var body struct {
		ID       string `json:"id"`
		PIN      string `json:"pin"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	se := s.session(r)
	if se == nil && !s.bypassApplies(r) {
		writeError(w, http.StatusUnauthorized, "sign in first")
		return
	}
	u, err := s.store.GetUser(ctx, body.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if u == nil {
		writeError(w, http.StatusNotFound, "no such profile")
		return
	}
	if u.IsFriend || (se != nil && se.Via == "friend") {
		writeError(w, http.StatusForbidden, "friends sign in with their own name and password")
		return
	}
	key := "profile:" + clientIP(r) + ":" + u.ID
	if !s.limiter.Allow(key) {
		writeError(w, http.StatusTooManyRequests, auth.ErrTooManyAttempts.Error())
		return
	}
	passwordSession := se != nil && se.Via == "password"
	switch {
	case u.HasPIN:
		if !auth.Check(u.PINHash, body.PIN) {
			s.limiter.Fail(key)
			s.log.Warn("profile sign-in failed: wrong PIN", "profile", u.Name, "address", clientIP(r))
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "wrong PIN", "needs": "pin"})
			return
		}
	case u.IsAdmin && !passwordSession:
		if !auth.Check(u.PasswordHash, body.Password) {
			if body.Password != "" {
				s.limiter.Fail(key)
				s.log.Warn("profile sign-in failed: wrong password", "profile", u.Name, "address", clientIP(r))
			}
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "enter the password", "needs": "password"})
			return
		}
		passwordSession = true
	}
	s.limiter.Succeed(key)

	switch {
	case se == nil || (passwordSession && se.Via != "password"):
		// No session yet, or the password was just given: start a fresh one.
		if se != nil {
			s.store.DeleteSession(ctx, se.TokenHash)
		}
		via := "local"
		if passwordSession {
			via = "password"
		}
		if _, err := s.startSession(w, r, u.ID, via); err != nil {
			s.fail(w, err)
			return
		}
	default:
		if err := s.store.SetSessionUser(ctx, se.TokenHash, u.ID); err != nil {
			s.fail(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, toProfile(*u))
}

// leaveProfile: DELETE /api/auth/profile goes back to the profile picker.
func (s *Server) leaveProfile(w http.ResponseWriter, r *http.Request) {
	if se := s.session(r); se != nil {
		// A friend has only their own profile: leaving it signs them out.
		if se.Via == "friend" {
			s.logout(w, r)
			return
		}
		if err := s.store.SetSessionUser(r.Context(), se.TokenHash, ""); err != nil {
			s.fail(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// logout: POST /api/auth/logout signs this browser out.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if token := requestToken(r); token != "" {
		s.store.DeleteSession(r.Context(), auth.HashToken(token))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		Secure: secureCookie(r), SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

// changePassword: POST /api/auth/password {"current", "new"} (admin). Other browsers signed in with
// the old password are signed out.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	admin := currentUser(r)
	key := "password:" + clientIP(r)
	if !s.limiter.Allow(key) {
		writeError(w, http.StatusTooManyRequests, auth.ErrTooManyAttempts.Error())
		return
	}
	if !auth.Check(admin.PasswordHash, body.Current) {
		s.limiter.Fail(key)
		writeError(w, http.StatusUnauthorized, "the current password isn't right")
		return
	}
	s.limiter.Succeed(key)
	if len(body.New) < auth.MinPasswordLength {
		writeError(w, http.StatusBadRequest, "the new password needs at least 8 characters")
		return
	}
	hash, err := auth.Hash(body.New)
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.store.UpdateUser(r.Context(), admin.ID, store.UserUpdate{PasswordHash: &hash}); err != nil {
		s.fail(w, err)
		return
	}
	keep := ""
	if se := s.session(r); se != nil {
		keep = se.TokenHash
	}
	if err := s.store.DeletePasswordSessions(r.Context(), keep); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listProfiles: GET /api/profiles. Profiles.
func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]profileDTO, 0, len(users))
	for _, u := range users {
		out = append(out, toProfile(u))
	}
	writeJSON(w, http.StatusOK, out)
}

type profileBody struct {
	Name *string `json:"name"`
	// Make a friend (with an invite link) rather than a household profile.
	Friend bool    `json:"friend"`
	Color  *string `json:"color"`
	// A four-digit PIN; "" removes it.
	PIN *string `json:"pin"`
	// Age limit: 0 (none), or the oldest age rating allowed.
	MaxAge *int `json:"max_age"`
	// The libraries the profile may see: a list of ids, or null for all of them. Leave the field
	// out to keep them as they are.
	LibraryIDs json.RawMessage `json:"library_ids"`
	// Labels whose titles the profile doesn't see.
	ExcludeLabels *[]string `json:"exclude_labels"`
}

// libraries reads library_ids from a profile body: whether it was sent, and the list (nil for all).
func (b *profileBody) libraries() (sent bool, ids []int64, err error) {
	if len(b.LibraryIDs) == 0 {
		return false, nil, nil
	}
	if string(b.LibraryIDs) == "null" {
		return true, nil, nil
	}
	ids = []int64{}
	if err := json.Unmarshal(b.LibraryIDs, &ids); err != nil {
		return true, nil, errors.New("library_ids is a list of library ids, or null for all")
	}
	return true, ids, nil
}

func (b *profileBody) validate() error {
	if b.Name != nil {
		n := strings.TrimSpace(*b.Name)
		if n == "" || len(n) > 40 {
			return errors.New("enter a name of up to 40 characters")
		}
		b.Name = &n
	}
	if b.Color != nil && !profileColors[*b.Color] {
		return errors.New("unknown colour")
	}
	if b.PIN != nil && *b.PIN != "" && !auth.ValidPIN(*b.PIN) {
		return errors.New("a PIN is four digits")
	}
	if b.MaxAge != nil && (*b.MaxAge < 0 || *b.MaxAge > 21) {
		return errors.New("max_age is 0 (no limit) or an age up to 21")
	}
	return nil
}

func (b *profileBody) pinHash() (*string, error) {
	if b.PIN == nil {
		return nil, nil
	}
	if *b.PIN == "" {
		empty := ""
		return &empty, nil
	}
	h, err := auth.Hash(*b.PIN)
	return &h, err
}

// createProfile: POST /api/profiles {"name", "color", "pin"} (admin)
func (s *Server) createProfile(w http.ResponseWriter, r *http.Request) {
	var b profileBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if b.Name == nil {
		writeError(w, http.StatusBadRequest, "enter a name")
		return
	}
	if err := b.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	color := "blue"
	if b.Color != nil {
		color = *b.Color
	}
	pin, err := b.pinHash()
	if err != nil {
		s.fail(w, err)
		return
	}
	pinHash := ""
	if pin != nil {
		pinHash = *pin
	}
	u, err := s.store.CreateUser(r.Context(), *b.Name, false, color, "", pinHash)
	if errors.Is(err, store.ErrNameTaken) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	up := store.UserUpdate{}
	if b.MaxAge != nil && *b.MaxAge > 0 {
		up.MaxAge = b.MaxAge
		u.MaxAge = *b.MaxAge
	}
	if sent, ids, err := b.libraries(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	} else if sent && ids != nil {
		up.LibraryIDs = &ids
		u.LibraryIDs = ids
	}
	if b.ExcludeLabels != nil {
		labels := cleanLabels(*b.ExcludeLabels)
		up.ExcludeLabels = &labels
		u.ExcludeLabels = labels
	}
	if up.MaxAge != nil || up.LibraryIDs != nil || up.ExcludeLabels != nil {
		if err := s.store.UpdateUser(r.Context(), u.ID, up); err != nil {
			s.fail(w, err)
			return
		}
	}
	out := toProfile(*u)
	if b.Friend {
		token, err := s.newInvite(r, u.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		out.IsFriend, out.InviteToken = true, token
	}
	writeJSON(w, http.StatusCreated, out)
}

// updateProfile: PATCH /api/profiles/{id} {"name", "color", "pin"} (admin)
func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if u == nil {
		writeError(w, http.StatusNotFound, "no such profile")
		return
	}
	var b profileBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := b.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	pin, err := b.pinHash()
	if err != nil {
		s.fail(w, err)
		return
	}
	if u.IsAdmin && b.MaxAge != nil && *b.MaxAge != 0 {
		writeError(w, http.StatusBadRequest, "the admin profile can't have an age limit")
		return
	}
	up := store.UserUpdate{Name: b.Name, Color: b.Color, PINHash: pin, MaxAge: b.MaxAge}
	sent, ids, err := b.libraries()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if sent && u.IsAdmin && ids != nil {
		writeError(w, http.StatusBadRequest, "the admin profile sees every library")
		return
	}
	if sent {
		up.LibraryIDs, up.AllLibraries = &ids, ids == nil
	}
	if b.ExcludeLabels != nil {
		if u.IsAdmin && len(*b.ExcludeLabels) > 0 {
			writeError(w, http.StatusBadRequest, "the admin profile sees every title")
			return
		}
		labels := cleanLabels(*b.ExcludeLabels)
		up.ExcludeLabels = &labels
	}
	err = s.store.UpdateUser(r.Context(), u.ID, up)
	if errors.Is(err, store.ErrNameTaken) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	u, err = s.store.GetUser(r.Context(), u.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toProfile(*u))
}

// deleteProfile: DELETE /api/profiles/{id} (admin; the admin profile can't be removed) deletes a
// profile and its watch history.
func (s *Server) deleteProfile(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if u == nil {
		writeError(w, http.StatusNotFound, "no such profile")
		return
	}
	if u.IsAdmin {
		writeError(w, http.StatusBadRequest, "the admin profile can't be removed")
		return
	}
	if err := s.store.DeleteUser(r.Context(), u.ID); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clientIP is the request's address without the port (through a trusted proxy, the client's),
// for rate limiting, logs and the Dashboard.
func clientIP(r *http.Request) string { return auth.ClientIP(r) }

// settingsDTO is what the Settings page shows. API keys are only hinted at.
func (s *Server) settingsDTO(st settings.Settings) map[string]any {
	ff := s.ffprobeStatus()
	return map[string]any{
		"tmdb_api_key": maskKey(st.TMDBKey),
		"tvdb_api_key": maskKey(st.TVDBKey),
		"ffprobe_path": st.FFprobePath,
		"ffprobe":      ff,
		"local_bypass": st.LocalBypass,
		"hw_accel":     st.HWAccel,
		"tone_mapping": st.ToneMap,
		// The password is never sent back; only whether there's an account.
		"opensubtitles_api_key":  maskKey(st.OpenSubtitlesKey),
		"opensubtitles_username": st.OpenSubtitlesUser,
		"port":                   st.Port,
		"allow_network":          st.AllowNetwork,
		"region":                 st.Region,
		"scan_automatic":         st.ScanAutomatic,
		"scan_partial":           st.ScanPartial,
		"scan_on_start":          st.ScanOnStart,
		"scan_interval_hours":    float64(st.ScanMinutes()) / 60,
		"scan_interval_minutes":  st.ScanMinutes(), "analysis_enabled": st.AnalysisEnabled, "metadata_refresh_enabled": st.MetadataRefreshEnabled, "auto_backups_enabled": st.AutoBackupsEnabled, "empty_trash_enabled": st.EmptyTrashEnabled, "database_upkeep_enabled": st.DatabaseUpkeepEnabled, "cache_cleanup_enabled": st.CacheCleanupEnabled, "preview_mode": st.PreviewMode, "detection_mode": st.DetectionMode,
		"maintenance_start":  st.MaintenanceStart,
		"maintenance_end":    st.MaintenanceEnd,
		"previews_at_night":  st.PreviewsAtNight,
		"omdb_api_key":       maskKey(st.OMDbKey),
		"preroll_paths":      st.PrerollPaths,
		"preroll_mode":       st.PrerollMode,
		"preroll_found":      len(prerollFiles(st.PrerollPaths)),
		"theintrodb_api_key": maskKey(st.TheIntroDBKey),
		"detect_intros":      st.DetectIntros,
		"remote_kbps":        st.RemoteKbps,
		"upload_kbps":        st.UploadKbps,
		"remote_access":      st.RemoteAccess,
		"remote_port":        st.OutsidePort(),
		"upnp":               st.UPnP,
		"https_mode":         st.HTTPSMode,
		"https_cert":         st.HTTPSCert,
		"https_key":          st.HTTPSKey,
		"https_domain":       st.HTTPSDomain,
		"home_networks":      st.HomeNetworks,
		"trusted_proxies":    st.TrustedProxies,
		"transcode_dir":      st.TranscodeDir,
		"max_transcodes":     st.MaxTranscodes,
		"listen":             st.ListenAddress(),
		"from_env":           settings.FromEnv(),
		"transcoder":         s.transcoderCaps(),
	}
}

// writableDir checks Arc can write in a folder (made if it isn't there yet).
func writableDir(dir string) error {
	if !filepath.IsAbs(dir) {
		return errors.New("choose a full path, such as D:\\Arc Transcode")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("can't use %s: %w", dir, err)
	}
	f, err := os.CreateTemp(dir, ".arc-check-*")
	if err != nil {
		return fmt.Errorf("Arc can't write in %s", dir)
	}
	f.Close()
	os.Remove(f.Name())
	return nil
}

func maskKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 4 {
		return "••••"
	}
	return "••••" + k[len(k)-4:]
}

// getSettings: GET /api/settings. Server settings: API keys, ffprobe, sign-in at home, port,
// network, `maintenance_start`/`maintenance_end` (hours) and `previews_at_night`; remote access:
// `remote_access`, `remote_port`, `upnp`, `https_mode` (`""`, `files`, `letsencrypt`),
// `https_cert`, `https_key`, `https_domain`; `home_networks` (e.g. `"100.64.0.0/10, 10.8.0.0/24"`);
// `transcode_dir` (a full path, `""` for the data folder), `max_transcodes` (0 for no limit).
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.settingsDTO(s.settings()))
}

// updateSettings: PATCH /api/settings (admin) with any of "tmdb_api_key", "tvdb_api_key",
// "ffprobe_path", "local_bypass", "port", "allow_network". Keys are checked with the provider
// before being saved; a new port or network setting takes effect straight away.
// Global scanning: "scan_automatic", "scan_partial", "scan_on_start" (booleans), and
// "scan_interval_minutes" (0, 15, 30, 60, 120, 360, 720 or 1440), legacy
// "scan_interval_hours", processing modes (off/scheduled/added) and task enable switches.
// Changes apply without restarting.
// logSettingChanges records which server settings a successful save changed, and by whom.
// Keys and passwords are logged only as "(changed)".
func (s *Server) logSettingChanges(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		rec := &settingsRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)
		var changed map[string]json.RawMessage
		if rec.status >= 300 || json.Unmarshal(body, &changed) != nil || len(changed) == 0 {
			return
		}
		parts := make([]string, 0, len(changed))
		for k, v := range changed {
			value := string(v)
			if strings.Contains(k, "key") || strings.Contains(k, "password") || len(value) > 80 {
				value = "(changed)"
			}
			parts = append(parts, k+"="+value)
		}
		slices.Sort(parts)
		name := ""
		if u := currentUser(r); u != nil {
			name = u.Name
		}
		s.log.Info("settings changed", "by", name, "settings", strings.Join(parts, ", "))
	}
}

type settingsRecorder struct {
	http.ResponseWriter
	status int
}

func (r *settingsRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TMDBKey                *string  `json:"tmdb_api_key"`
		TVDBKey                *string  `json:"tvdb_api_key"`
		FFprobePath            *string  `json:"ffprobe_path"`
		LocalBypass            *bool    `json:"local_bypass"`
		HWAccel                *bool    `json:"hw_accel"`
		ToneMap                *bool    `json:"tone_mapping"`
		OSKey                  *string  `json:"opensubtitles_api_key"`
		OSUser                 *string  `json:"opensubtitles_username"`
		OSPassword             *string  `json:"opensubtitles_password"`
		Port                   *int     `json:"port"`
		AllowNetwork           *bool    `json:"allow_network"`
		Region                 *string  `json:"region"`
		ScanAutomatic          *bool    `json:"scan_automatic"`
		ScanPartial            *bool    `json:"scan_partial"`
		ScanOnStart            *bool    `json:"scan_on_start"`
		ScanInterval           *float64 `json:"scan_interval_hours"`
		ScanMinutes            *int     `json:"scan_interval_minutes"`
		AnalysisEnabled        *bool    `json:"analysis_enabled"`
		MetadataRefreshEnabled *bool    `json:"metadata_refresh_enabled"`
		AutoBackupsEnabled     *bool    `json:"auto_backups_enabled"`
		EmptyTrashEnabled      *bool    `json:"empty_trash_enabled"`
		DatabaseUpkeepEnabled  *bool    `json:"database_upkeep_enabled"`
		CacheCleanupEnabled    *bool    `json:"cache_cleanup_enabled"`
		PreviewMode            *string  `json:"preview_mode"`
		DetectionMode          *string  `json:"detection_mode"`
		MaintStart             *int     `json:"maintenance_start"`
		MaintEnd               *int     `json:"maintenance_end"`
		PreviewsNight          *bool    `json:"previews_at_night"`
		OMDbKey                *string  `json:"omdb_api_key"`
		PrerollPaths           *string  `json:"preroll_paths"`
		PrerollMode            *string  `json:"preroll_mode"`
		TheIntroDBKey          *string  `json:"theintrodb_api_key"`
		DetectIntros           *bool    `json:"detect_intros"`
		RemoteKbps             *int     `json:"remote_kbps"`
		UploadKbps             *int     `json:"upload_kbps"`
		RemoteAccess           *bool    `json:"remote_access"`
		RemotePort             *int     `json:"remote_port"`
		UPnP                   *bool    `json:"upnp"`
		HTTPSMode              *string  `json:"https_mode"`
		HTTPSCert              *string  `json:"https_cert"`
		HTTPSKey               *string  `json:"https_key"`
		HTTPSDomain            *string  `json:"https_domain"`
		HomeNetworks           *string  `json:"home_networks"`
		TrustedProxies         *string  `json:"trusted_proxies"`
		TranscodeDir           *string  `json:"transcode_dir"`
		MaxTranscodes          *int     `json:"max_transcodes"`
	}
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	for _, key := range []string{"scan_automatic", "scan_partial", "scan_on_start", "scan_interval_hours", "scan_interval_minutes", "preview_mode", "detection_mode", "analysis_enabled", "metadata_refresh_enabled", "auto_backups_enabled", "empty_trash_enabled", "database_upkeep_enabled", "cache_cleanup_enabled"} {
		if v, ok := fields[key]; ok && string(v) == "null" {
			writeError(w, http.StatusBadRequest, key+" cannot be null")
			return
		}
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	next := s.settings()
	if body.ScanAutomatic != nil {
		next.ScanAutomatic = *body.ScanAutomatic
	}
	if body.ScanPartial != nil {
		next.ScanPartial = *body.ScanPartial
	}
	if body.ScanOnStart != nil {
		next.ScanOnStart = *body.ScanOnStart
	}
	if body.ScanInterval != nil {
		minutes := *body.ScanInterval * 60
		if minutes != float64(int(minutes)) || !settings.ValidScanMinutes(int(minutes)) {
			writeError(w, 400, "unsupported scan interval")
			return
		}
		next.ScanIntervalMinutes = int(minutes)
		next.ScanIntervalHours = int(minutes) / 60
	}
	if body.ScanMinutes != nil {
		if !settings.ValidScanMinutes(*body.ScanMinutes) {
			writeError(w, 400, "scan interval must be 0, 15, 30, 60, 120, 360, 720 or 1440 minutes")
			return
		}
		next.ScanIntervalMinutes = *body.ScanMinutes
		next.ScanIntervalHours = *body.ScanMinutes / 60
	}
	for _, p := range []struct {
		src  *string
		dest *string
	}{{body.PreviewMode, &next.PreviewMode}, {body.DetectionMode, &next.DetectionMode}} {
		if p.src != nil {
			if !settings.ValidProcessingMode(*p.src) {
				writeError(w, 400, "processing mode must be off, scheduled or added")
				return
			}
			*p.dest = *p.src
		}
	}
	if body.AnalysisEnabled != nil {
		next.AnalysisEnabled = *body.AnalysisEnabled
	}
	if body.MetadataRefreshEnabled != nil {
		next.MetadataRefreshEnabled = *body.MetadataRefreshEnabled
	}
	if body.DatabaseUpkeepEnabled != nil {
		next.DatabaseUpkeepEnabled = *body.DatabaseUpkeepEnabled
	}
	if body.CacheCleanupEnabled != nil {
		next.CacheCleanupEnabled = *body.CacheCleanupEnabled
	}
	if body.EmptyTrashEnabled != nil {
		next.EmptyTrashEnabled = *body.EmptyTrashEnabled
	}
	if body.AutoBackupsEnabled != nil {
		next.AutoBackupsEnabled = *body.AutoBackupsEnabled
	}
	if body.TMDBKey != nil {
		next.TMDBKey = strings.TrimSpace(*body.TMDBKey)
	}
	if body.TVDBKey != nil {
		next.TVDBKey = strings.TrimSpace(*body.TVDBKey)
	}
	if body.FFprobePath != nil {
		next.FFprobePath = strings.TrimSpace(*body.FFprobePath)
	}
	if body.LocalBypass != nil {
		next.LocalBypass = *body.LocalBypass
	}
	if body.HWAccel != nil {
		next.HWAccel = *body.HWAccel
	}
	if body.ToneMap != nil {
		next.ToneMap = *body.ToneMap
	}
	if body.OSKey != nil {
		next.OpenSubtitlesKey = strings.TrimSpace(*body.OSKey)
	}
	if body.OSUser != nil {
		next.OpenSubtitlesUser = strings.TrimSpace(*body.OSUser)
		if next.OpenSubtitlesUser == "" {
			next.OpenSubtitlesPassword = ""
		}
	}
	if body.OSPassword != nil {
		next.OpenSubtitlesPassword = *body.OSPassword
	}
	if body.Port != nil {
		if *body.Port < 1024 || *body.Port > 65535 {
			writeError(w, http.StatusBadRequest, "choose a port between 1024 and 65535")
			return
		}
		next.Port = *body.Port
	}
	if body.AllowNetwork != nil {
		next.AllowNetwork = *body.AllowNetwork
	}
	if body.Region != nil {
		if !regionRe.MatchString(*body.Region) {
			writeError(w, http.StatusBadRequest, "region is a two-letter country code such as GB")
			return
		}
		next.Region = *body.Region
	}
	for _, h := range []*int{body.MaintStart, body.MaintEnd} {
		if h != nil && (*h < 0 || *h > 23) {
			writeError(w, http.StatusBadRequest, "maintenance hours are 0 to 23")
			return
		}
	}
	if body.MaintStart != nil {
		next.MaintenanceStart = *body.MaintStart
	}
	if body.MaintEnd != nil {
		next.MaintenanceEnd = *body.MaintEnd
	}
	if body.PreviewsNight != nil {
		next.PreviewsAtNight = *body.PreviewsNight
		if body.PreviewMode == nil {
			next.PreviewMode = "added"
			if *body.PreviewsNight {
				next.PreviewMode = "scheduled"
			}
		}
	}
	if body.PreviewMode != nil {
		next.PreviewsAtNight = next.PreviewMode == "scheduled"
	}
	if body.DetectionMode != nil {
		next.DetectIntros = next.DetectionMode != "off"
	}
	if body.TheIntroDBKey != nil {
		next.TheIntroDBKey = strings.TrimSpace(*body.TheIntroDBKey)
	}
	if body.DetectIntros != nil {
		next.DetectIntros = *body.DetectIntros
		if body.DetectionMode == nil {
			next.DetectionMode = "off"
			if *body.DetectIntros {
				next.DetectionMode = "scheduled"
			}
		}
	}
	for _, v := range []*int{body.RemoteKbps, body.UploadKbps} {
		if v != nil && (*v < 0 || *v > 1_000_000) {
			writeError(w, http.StatusBadRequest, "speeds are in kbps, 0 for no limit")
			return
		}
	}
	if body.RemoteKbps != nil {
		next.RemoteKbps = *body.RemoteKbps
	}
	if body.UploadKbps != nil {
		next.UploadKbps = *body.UploadKbps
	}
	if body.TranscodeDir != nil {
		dir := strings.TrimSpace(*body.TranscodeDir)
		if dir != "" {
			if err := writableDir(dir); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		next.TranscodeDir = dir
	}
	if body.MaxTranscodes != nil {
		if *body.MaxTranscodes < 0 || *body.MaxTranscodes > 50 {
			writeError(w, http.StatusBadRequest, "max_transcodes is 0 (no limit) to 50")
			return
		}
		next.MaxTranscodes = *body.MaxTranscodes
	}
	if body.HomeNetworks != nil {
		_, tidy, err := auth.ParseNetworks(*body.HomeNetworks)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		next.HomeNetworks = tidy
	}
	if body.TrustedProxies != nil {
		_, tidy, err := auth.ParseNetworks(*body.TrustedProxies)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		next.TrustedProxies = tidy
	}
	if body.RemoteAccess != nil {
		next.RemoteAccess = *body.RemoteAccess
	}
	if body.RemotePort != nil {
		if *body.RemotePort < 1 || *body.RemotePort > 65535 {
			writeError(w, http.StatusBadRequest, "choose an outside port between 1 and 65535")
			return
		}
		next.RemotePort = *body.RemotePort
		if next.RemotePort == next.Port {
			next.RemotePort = 0 // the same as Arc's port, following it if that changes
		}
	}
	if body.UPnP != nil {
		next.UPnP = *body.UPnP
	}
	if body.HTTPSMode != nil {
		next.HTTPSMode = *body.HTTPSMode
	}
	if body.HTTPSCert != nil {
		next.HTTPSCert = strings.TrimSpace(*body.HTTPSCert)
	}
	if body.HTTPSKey != nil {
		next.HTTPSKey = strings.TrimSpace(*body.HTTPSKey)
	}
	if body.HTTPSDomain != nil {
		next.HTTPSDomain = strings.ToLower(strings.TrimSpace(*body.HTTPSDomain))
	}
	if err := remote.Check(next.HTTPSMode, next.HTTPSCert, next.HTTPSKey, next.HTTPSDomain); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.PrerollPaths != nil {
		next.PrerollPaths = strings.TrimSpace(*body.PrerollPaths)
	}
	if body.PrerollMode != nil {
		if *body.PrerollMode != "all" && *body.PrerollMode != "random" {
			writeError(w, http.StatusBadRequest, `preroll_mode is "random" or "all"`)
			return
		}
		next.PrerollMode = *body.PrerollMode
	}
	if body.OMDbKey != nil {
		next.OMDbKey = strings.TrimSpace(*body.OMDbKey)
	}
	if err := s.applySettings(r.Context(), next); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.settingsDTO(s.settings()))
}

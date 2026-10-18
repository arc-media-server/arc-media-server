package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"ams/internal/auth"
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
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	se, err := s.store.GetSession(r.Context(), auth.HashToken(c.Value))
	if err != nil || se == nil {
		return nil
	}
	if se.Via == "local" && !s.bypassApplies(r) {
		return nil
	}
	if time.Since(time.Unix(se.LastSeen, 0)) > 5*time.Minute {
		s.store.TouchSession(r.Context(), se.TokenHash)
	}
	return se
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

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: 365 * 24 * 3600})
}

// startSession creates a session and sets its cookie.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID, via string) (string, error) {
	token, hash := auth.NewToken()
	if err := s.store.CreateSession(r.Context(), hash, userID, via, r.UserAgent()); err != nil {
		return "", err
	}
	s.setSessionCookie(w, token)
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
}

func toProfile(u store.User) profileDTO {
	return profileDTO{ID: u.ID, Name: u.Name, Color: u.Color, HasPIN: u.HasPIN, IsAdmin: u.IsAdmin, MaxAge: u.MaxAge, LibraryIDs: u.LibraryIDs, ExcludeLabels: u.ExcludeLabels}
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
	if se != nil || s.bypassApplies(r) {
		users, err := s.store.ListUsers(ctx)
		if err != nil {
			s.fail(w, err)
			return
		}
		profiles := make([]profileDTO, 0, len(users))
		for _, u := range users {
			profiles = append(profiles, toProfile(u))
		}
		out["profiles"] = profiles
	}
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
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	key := "login:" + clientIP(r)
	if !s.limiter.Allow(key) {
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
		writeError(w, http.StatusUnauthorized, "that password isn't right")
		return
	}
	s.limiter.Succeed(key)
	if old := s.session(r); old != nil {
		s.store.DeleteSession(r.Context(), old.TokenHash)
	}
	if _, err := s.startSession(w, r, "", "password"); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "wrong PIN", "needs": "pin"})
			return
		}
	case u.IsAdmin && !passwordSession:
		if !auth.Check(u.PasswordHash, body.Password) {
			if body.Password != "" {
				s.limiter.Fail(key)
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
		if err := s.store.SetSessionUser(r.Context(), se.TokenHash, ""); err != nil {
			s.fail(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// logout: POST /api/auth/logout signs this browser out.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.store.DeleteSession(r.Context(), auth.HashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
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

// listProfiles: GET /api/profiles (admin)
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
	Name  *string `json:"name"`
	Color *string `json:"color"`
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
	writeJSON(w, http.StatusCreated, toProfile(*u))
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

// clientIP is the request's address without the port, for rate limiting.
func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}

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
		"maintenance_start":      st.MaintenanceStart,
		"maintenance_end":        st.MaintenanceEnd,
		"previews_at_night":      st.PreviewsAtNight,
		"listen":                 st.ListenAddress(),
		"from_env":               settings.FromEnv(),
		"transcoder":             s.transcoderCaps(),
	}
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

// getSettings: GET /api/settings (admin)
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.settingsDTO(s.settings()))
}

// updateSettings: PATCH /api/settings (admin) with any of "tmdb_api_key", "tvdb_api_key",
// "ffprobe_path", "local_bypass", "port", "allow_network". Keys are checked with the provider
// before being saved; a new port or network setting takes effect straight away.
func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TMDBKey       *string `json:"tmdb_api_key"`
		TVDBKey       *string `json:"tvdb_api_key"`
		FFprobePath   *string `json:"ffprobe_path"`
		LocalBypass   *bool   `json:"local_bypass"`
		HWAccel       *bool   `json:"hw_accel"`
		ToneMap       *bool   `json:"tone_mapping"`
		OSKey         *string `json:"opensubtitles_api_key"`
		OSUser        *string `json:"opensubtitles_username"`
		OSPassword    *string `json:"opensubtitles_password"`
		Port          *int    `json:"port"`
		AllowNetwork  *bool   `json:"allow_network"`
		Region        *string `json:"region"`
		MaintStart    *int    `json:"maintenance_start"`
		MaintEnd      *int    `json:"maintenance_end"`
		PreviewsNight *bool   `json:"previews_at_night"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	next := s.settings()
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
	}
	if err := s.applySettings(r.Context(), next); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.settingsDTO(s.settings()))
}

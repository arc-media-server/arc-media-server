package api

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"ams/internal/auth"
)

// Apps sign in with a token in the Authorization header. A TV, where typing a password is a
// chore, gets one with a code instead, as with plex.tv/link: the TV shows a short code, someone
// enters it in the web app (Profiles menu > Link a Device, or /link), and the TV is signed in.

// linkLife is how long a TV's code works.
const linkLife = 10 * time.Minute

type linkRequest struct {
	code    string
	device  string
	expires time.Time
	token   string // set once approved, until the TV collects it
}

type linkCodes struct {
	mu     sync.Mutex
	byPoll map[string]*linkRequest
}

// Codes leave out letters and digits that are easy to mix up (0/O, 1/I/L).
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func newCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b)
}

func (l *linkCodes) add(device string) (code, poll string, ok bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.byPoll == nil {
		l.byPoll = map[string]*linkRequest{}
	}
	for p, req := range l.byPoll {
		if time.Now().After(req.expires) {
			delete(l.byPoll, p)
		}
	}
	if len(l.byPoll) >= 100 {
		return "", "", false
	}
	for {
		code = newCode()
		taken := false
		for _, req := range l.byPoll {
			taken = taken || req.code == code
		}
		if !taken {
			break
		}
	}
	poll, _ = auth.NewToken()
	l.byPoll[poll] = &linkRequest{code: code, device: device, expires: time.Now().Add(linkLife)}
	return code, poll, true
}

// pending finds an unapproved code's request.
func (l *linkCodes) pending(code string) *linkRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	code = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), " ", ""))
	for _, req := range l.byPoll {
		if req.code == code && req.token == "" && time.Now().Before(req.expires) {
			return req
		}
	}
	return nil
}

// startLink: POST /api/auth/link {"device"} (anyone) gives a TV a code to show, and a poll token
// to ask with whether it's been approved.
func (s *Server) startLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Device string `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Device) == "" {
		writeError(w, http.StatusBadRequest, `send {"device": "a name for this device"}`)
		return
	}
	code, poll, ok := s.links.add(strings.TrimSpace(body.Device))
	if !ok {
		writeError(w, http.StatusTooManyRequests, "too many devices are waiting to be linked; try again in a few minutes")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": code, "poll": poll, "expires_in": int(linkLife.Seconds())})
}

// pollLink: GET /api/auth/link/{poll} (anyone) answers 202 while the code waits, then once with
// {"token"} after it's approved; 404 when it has run out.
func (s *Server) pollLink(w http.ResponseWriter, r *http.Request) {
	s.links.mu.Lock()
	req := s.links.byPoll[r.PathValue("poll")]
	if req != nil && req.token != "" {
		delete(s.links.byPoll, r.PathValue("poll"))
	}
	s.links.mu.Unlock()
	switch {
	case req == nil || (req.token == "" && time.Now().After(req.expires)):
		writeError(w, http.StatusNotFound, "this code has run out; ask for a new one")
	case req.token == "":
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "waiting"})
	default:
		writeJSON(w, http.StatusOK, map[string]string{"token": req.token})
	}
}

// approveLink: POST /api/auth/link/approve {"code"} signs the TV showing the code in, as this
// household (it then picks a profile) or, for a friend, as them. The household needs the password
// to have been used here (or the admin profile), so someone at home without it can't give a device
// a sign-in that works away from home.
func (s *Server) approveLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	key := "link:" + clientIP(r)
	if !s.limiter.Allow(key) {
		writeError(w, http.StatusTooManyRequests, auth.ErrTooManyAttempts.Error())
		return
	}
	req := s.links.pending(body.Code)
	if req == nil {
		s.limiter.Fail(key)
		writeError(w, http.StatusNotFound, "that code isn't right, or has run out")
		return
	}
	se := s.session(r)
	u := currentUser(r)
	userID, via := "", "password"
	switch {
	case se != nil && se.Via == "friend":
		userID, via = u.ID, "friend"
	case se != nil && (se.Via == "password" || u.IsAdmin):
	default:
		writeError(w, http.StatusForbidden, "sign in with the password first (Sign Out, then sign in) to link a device")
		return
	}
	token, err := s.deviceSession(r, userID, via, req.device)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.links.mu.Lock()
	req.token = token
	req.expires = time.Now().Add(linkLife) // time for the TV to collect it
	s.links.mu.Unlock()
	s.limiter.Succeed(key)
	s.log.Info("device linked", "device", req.device, "by", u.Name)
	writeJSON(w, http.StatusOK, map[string]string{"device": req.device})
}

// listDevices: GET /api/devices (admin) lists the signed-in browsers and apps.
func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListSessions(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	current := currentSession(r)
	out := make([]map[string]any, 0, len(list))
	for _, si := range list {
		out = append(out, map[string]any{"id": si.TokenHash, "device": si.Device, "user_agent": si.UserAgent, "via": si.Via,
			"profile": si.UserName, "created_at": si.CreatedAt, "last_seen_at": si.LastSeen, "this_device": si.TokenHash == current})
	}
	writeJSON(w, http.StatusOK, out)
}

// signOutDevice: DELETE /api/devices/{id} (admin) signs a browser or app out.
func (s *Server) signOutDevice(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteSession(r.Context(), r.PathValue("id")); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"ams/internal/auth"
)

// Casting: a Chromecast or Apple TV fetches the video itself, without the browser's sign-in
// cookie. The player swaps its stream URL for a cast link carrying a token in the path (so the
// relative segment links inside HLS playlists keep it). The token stands in for the browser's
// session, so the dashboard shows one playback and stopping it stops the cast too.

// castLife is how long a cast link works after it was last used.
const castLife = 6 * time.Hour

type castGrant struct {
	userID  string
	session string // the sender's session token hash
	itemID  string
	used    time.Time
}

type castTokens struct {
	mu     sync.Mutex
	grants map[string]*castGrant // by token hash
}

func (c *castTokens) add(g *castGrant) string {
	token, hash := auth.NewToken()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.grants == nil {
		c.grants = map[string]*castGrant{}
	}
	for h, old := range c.grants {
		if time.Since(old.used) > castLife {
			delete(c.grants, h)
		}
	}
	g.used = time.Now()
	c.grants[hash] = g
	return token
}

func (c *castTokens) get(token string) *castGrant {
	c.mu.Lock()
	defer c.mu.Unlock()
	g := c.grants[auth.HashToken(token)]
	if g == nil || time.Since(g.used) > castLife {
		return nil
	}
	g.used = time.Now()
	copy := *g
	return &copy
}

// Stream links a cast link may stand in for.
var (
	castStreamRe    = regexp.MustCompile(`^/api/items/([0-9a-f]+)/stream$`)
	castTranscodeRe = regexp.MustCompile(`^/api/transcode/([A-Za-z0-9_-]+)/([A-Za-z0-9_.-]+)$`)
	// A text subtitle track as WebVTT, which a Chromecast shows itself.
	castSubtitleRe = regexp.MustCompile(`^/api/items/([0-9a-f]+)/subtitles/([A-Za-z0-9_-]+)/vtt$`)
	// A trailer or other extra (an item's extras[].stream_url), for players that can't send the
	// sign-in with a request (the Roku and LG/Samsung apps' players).
	castExtraRe = regexp.MustCompile(`^/api/extras/([0-9a-f]+)/stream$`)
)

// castExtraGrant prefixes a grant's itemID when it's for an extra, so it never equals an item's.
const castExtraGrant = "extra:"

// castLink: POST /api/cast {"url"} turns a stream URL from POST /api/items/{id}/playback into a
// link a cast device can open: absolute, at an address it can reach, with a token. It also takes a
// text subtitle track's WebVTT link (GET /api/items/{id}/subtitles/{track}/vtt) and an extra's
// stream link (GET /api/extras/{id}/stream).
func (s *Server) castLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	u, err := url.Parse(body.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "not a stream link")
		return
	}
	itemID := ""
	if m := castStreamRe.FindStringSubmatch(u.Path); m != nil {
		itemID = m[1]
	} else if m := castSubtitleRe.FindStringSubmatch(u.Path); m != nil {
		itemID = m[1]
	} else if m := castExtraRe.FindStringSubmatch(u.Path); m != nil {
		e, err := s.store.GetExtra(r.Context(), m[1])
		if err != nil {
			s.fail(w, err)
			return
		}
		if e == nil {
			writeError(w, http.StatusNotFound, "extra not found")
			return
		}
		// The grant names the extra, so the link opens only that extra (not its title's file).
		itemID = castExtraGrant + e.ID
	} else if m := castTranscodeRe.FindStringSubmatch(u.Path); m != nil {
		sess := s.transcoder.Get(m[1])
		if sess == nil || sess.Options.Owner != currentSession(r) {
			writeError(w, http.StatusNotFound, "this playback has ended; start it again")
			return
		}
		itemID = sess.Options.ItemID
	} else {
		writeError(w, http.StatusBadRequest, "not a stream link")
		return
	}
	token := s.cast.add(&castGrant{userID: currentUser(r).ID, session: currentSession(r), itemID: itemID})
	link := s.castAddress(r) + "/api/cast/" + token + u.Path
	if u.RawQuery != "" {
		link += "?" + u.RawQuery
	}
	// Only this computer can reach Arc unless it accepts connections from other devices.
	host, _, _ := net.SplitHostPort(s.settings().ListenAddress())
	ip := net.ParseIP(host)
	writeJSON(w, http.StatusOK, map[string]any{"url": link, "local_only": ip != nil && ip.IsLoopback()})
}

// castAddress keeps direct LAN links local and uses the explicit public origin away from home.
func (s *Server) castAddress(r *http.Request) string {
	if cfg := s.settings(); cfg.RemoteAccess && cfg.PublicURL != "" && !auth.IsLocal(r) {
		return cfg.PublicURL
	}
	return castBase(r)
}

// castBase is this server's address as a device on the home network can reach it: the address
// the browser used, unless that's this computer's own name for itself.
func castBase(r *http.Request) string {
	scheme := "http"
	if auth.IsHTTPS(r) {
		scheme = "https"
	}
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host, port = r.Host, ""
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); host == "localhost" || (ip != nil && ip.IsLoopback()) {
		if lan := lanAddress(); lan != "" {
			host = lan
		}
	}
	if port != "" {
		return scheme + "://" + net.JoinHostPort(host, port)
	}
	return scheme + "://" + host
}

// lanAddress is this computer's address on the home network ("" if there's none).
func lanAddress() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ip, ok := a.(*net.IPNet); ok && ip.IP.To4() != nil && ip.IP.IsPrivate() {
			return ip.IP.String()
		}
	}
	return ""
}

// castStream: GET /api/cast/{token}/... serves a stream to a cast device.
func (s *Server) castStream(w http.ResponseWriter, r *http.Request) {
	// The device's player may fetch from another origin (Chromecast's receiver is a web page).
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Range")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	g := s.cast.get(r.PathValue("token"))
	if g == nil {
		writeError(w, http.StatusNotFound, "this cast link has run out; cast again")
		return
	}
	// Cast tokens inherit the sender's session; signing the device out, changing its profile or
	// disabling a home-only session must also prevent its next stream/segment request.
	se, err := s.store.GetSession(r.Context(), g.session)
	if err != nil || se == nil || se.UserID != g.userID || (se.Via == "local" && !s.bypassApplies(r)) {
		writeError(w, http.StatusUnauthorized, "this cast session has ended; sign in and cast again")
		return
	}
	u, err := s.store.GetUser(r.Context(), g.userID)
	if err != nil || u == nil {
		writeError(w, http.StatusNotFound, "this cast link has run out; cast again")
		return
	}
	ctx := context.WithValue(context.WithValue(r.Context(), userKey, u), sessionKey, g.session)
	r = r.WithContext(ctx)
	rest := "/" + r.PathValue("rest")
	if m := castStreamRe.FindStringSubmatch(rest); m != nil && m[1] == g.itemID {
		r.SetPathValue("id", m[1])
		s.stream(w, r)
		return
	}
	if m := castSubtitleRe.FindStringSubmatch(rest); m != nil && m[1] == g.itemID {
		r.SetPathValue("id", m[1])
		r.SetPathValue("track", m[2])
		s.subtitleVTT(w, r)
		return
	}
	if m := castExtraRe.FindStringSubmatch(rest); m != nil && castExtraGrant+m[1] == g.itemID {
		r.SetPathValue("id", m[1])
		s.streamExtra(w, r)
		return
	}
	if m := castTranscodeRe.FindStringSubmatch(rest); m != nil {
		r.SetPathValue("sid", m[1])
		r.SetPathValue("file", m[2])
		s.transcodeFile(w, r)
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

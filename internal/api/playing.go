package api

import (
	"ams/internal/auth"
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"ams/internal/store"
	"ams/internal/transcode"
)

// What's playing right now, for the admin dashboard. Nothing here is saved: a playback exists
// while its file is being streamed or the player keeps reporting progress.

const (
	// A viewing goes into watch history once it has played this long.
	minPlayMS = 60_000
	// Progress reports come every 10 seconds while playing; a longer gap (a stalled tab, a
	// sleeping laptop) counts as at most this much watching.
	maxReportGap = 30 * time.Second
	// A playback with no stream open and no progress report for this long has ended. The player
	// reports every 10 seconds while playing; a paused one goes quiet, so it's given longer.
	playingTimeout = 45 * time.Second
	pausedTimeout  = 30 * time.Minute
	// How long a stopped playback stays refused, so the browser can't simply reconnect.
	stopBlock = time.Minute
)

type playback struct {
	id         string
	key        string // session + what's playing
	userID     string
	itemID     string // the movie or episode, or the title an extra belongs to
	extraID    string
	device     string
	address    string
	positionMS int64
	durationMS int64
	paused     bool
	started    time.Time
	updated    time.Time
	streams    map[chan struct{}]bool // open stream requests, closed to cut them off
	// direct, remux or transcode, and the conversion's session if there is one.
	method    string
	sessionID string
	// Watch history: time spent playing so far, when it was last counted, whether it got to the
	// end, and its history row once it has one (adding is set while that row is being written).
	watchedMS  int64
	counted    time.Time
	completed  bool
	historyID  int64
	addingPlay bool
}

// playUpdate is what a progress report changes in watch history.
type playUpdate struct {
	key                               string
	add                               bool // the viewing is long enough to record: add its row
	historyID                         int64
	userID                            string
	started                           time.Time
	watchedMS, positionMS, durationMS int64
	completed                         bool
	// It reached the end with this report (for the playback.watched event).
	justCompleted  bool
	device, method string
}

type stopped struct {
	until   time.Time
	message string
}

type nowPlaying struct {
	mu      sync.Mutex
	byKey   map[string]*playback
	stopped map[string]stopped
}

func newNowPlaying() *nowPlaying {
	return &nowPlaying{byKey: map[string]*playback{}, stopped: map[string]stopped{}}
}

func playbackKey(r *http.Request, itemID, extraID string) string {
	return currentSession(r) + "|" + itemID + "|" + extraID
}

// refusal says why a stopped playback can't go on, or "" if it can. Call with the lock held.
func (n *nowPlaying) refusal(key string) string {
	st, ok := n.stopped[key]
	if !ok {
		return ""
	}
	if time.Now().After(st.until) {
		delete(n.stopped, key)
		return ""
	}
	if st.message != "" {
		return st.message
	}
	return "The server's admin stopped this."
}

// touch finds or starts the playback for a request. Call with the lock held.
func (n *nowPlaying) touch(r *http.Request, itemID, extraID string) *playback {
	key := playbackKey(r, itemID, extraID)
	p := n.byKey[key]
	now := time.Now()
	if p == nil {
		var b [8]byte
		rand.Read(b[:])
		p = &playback{id: hex.EncodeToString(b[:]), key: key, userID: currentUserID(r), itemID: itemID,
			extraID: extraID, started: now, streams: map[chan struct{}]bool{}}
		n.byKey[key] = p
	}
	p.device, p.address, p.updated = deviceName(r.UserAgent()), clientIP(r), now
	return p
}

// openStream records a stream request. It returns a channel closed if the admin stops the
// playback and a function to call when the request is done, or a reason it's refused.
func (n *nowPlaying) openStream(r *http.Request, itemID, extraID string) (stop chan struct{}, done func(), refused string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if why := n.refusal(playbackKey(r, itemID, extraID)); why != "" {
		return nil, nil, why
	}
	p := n.touch(r, itemID, extraID)
	stop = make(chan struct{})
	p.streams[stop] = true
	return stop, func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		delete(p.streams, stop)
		p.updated = time.Now()
	}, ""
}

// progress records a player's report. state is "playing", "paused" or "stopped" (the player
// closed). It returns a reason if the admin has stopped the playback, and what to record in
// watch history (nil if nothing).
func (n *nowPlaying) progress(r *http.Request, itemID string, positionMS, durationMS int64, state string) (string, *playUpdate) {
	n.mu.Lock()
	defer n.mu.Unlock()
	key := playbackKey(r, itemID, "")
	if why := n.refusal(key); why != "" {
		return why, nil
	}
	p := n.touch(r, itemID, "")
	// Count the time since the last report if it was playing (not paused) in between.
	now := time.Now()
	if !p.counted.IsZero() && !p.paused {
		p.watchedMS += min(now.Sub(p.counted), maxReportGap).Milliseconds()
	}
	p.counted = now
	p.positionMS, p.durationMS, p.paused = positionMS, durationMS, state == "paused"
	justCompleted := false
	if durationMS > 0 && float64(positionMS) >= 0.9*float64(durationMS) && !p.completed {
		p.completed, justCompleted = true, true
	}
	if state == "stopped" {
		delete(n.byKey, key)
	}
	u := &playUpdate{key: key, historyID: p.historyID, userID: p.userID, started: p.started, watchedMS: p.watchedMS,
		positionMS: positionMS, durationMS: durationMS, completed: p.completed, justCompleted: justCompleted, device: p.device,
		method: cmp.Or(p.method, "direct")}
	switch {
	case p.historyID != 0:
	case p.addingPlay || p.watchedMS < minPlayMS:
		return "", nil // not long enough yet, or its row is being added
	default:
		p.addingPlay, u.add = true, true
	}
	return "", u
}

// played notes a playback's watch history row once it's added.
func (n *nowPlaying) played(key string, id int64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if p := n.byKey[key]; p != nil {
		p.historyID, p.addingPlay = id, false
	}
}

// refusalFor says why the admin stopped this player playing an item, or "".
func (n *nowPlaying) refusalFor(r *http.Request, itemID string) string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.refusal(playbackKey(r, itemID, ""))
}

// setMethod records how a player is playing an item, ending the conversion it was using before
// (after a seek or a change of audio track).
func (n *nowPlaying) setMethod(r *http.Request, itemID, method, sessionID string, tc *transcode.Manager) {
	n.mu.Lock()
	p := n.touch(r, itemID, "")
	old := p.sessionID
	p.method, p.sessionID = method, sessionID
	n.mu.Unlock()
	if old != "" && old != sessionID && tc != nil {
		tc.Stop(old)
	}
}

// stop ends a playback: its streams are cut off and the player is told why when it next asks. It
// returns the playback's conversion session, if any, for the caller to end.
func (n *nowPlaying) stop(id, message string) (found bool, sessionID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for key, p := range n.byKey {
		if p.id != id {
			continue
		}
		for ch := range p.streams {
			close(ch)
		}
		delete(n.byKey, key)
		n.stopped[key] = stopped{until: time.Now().Add(stopBlock), message: message}
		return true, p.sessionID
	}
	return false, ""
}

// list returns the current playbacks, oldest first, forgetting those that have ended.
func (n *nowPlaying) list() []playback {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := time.Now()
	out := []playback{}
	for key, p := range n.byKey {
		timeout := playingTimeout
		if p.paused {
			timeout = pausedTimeout
		}
		if len(p.streams) == 0 && now.Sub(p.updated) > timeout {
			delete(n.byKey, key)
			continue
		}
		cp := *p
		cp.streams = maps.Clone(p.streams)
		out = append(out, cp)
	}
	slices.SortFunc(out, func(a, b playback) int { return a.started.Compare(b.started) })
	return out
}

// count is how many streams are open, for the dashboard's summary.
func (p *playback) streaming() bool { return len(p.streams) > 0 }

// stoppableFile cuts a stream off part-way when the admin stops it.
type stoppableFile struct {
	io.ReadSeeker
	stop chan struct{}
}

var errStopped = io.ErrUnexpectedEOF

func (f stoppableFile) Read(b []byte) (int, error) {
	select {
	case <-f.stop:
		return 0, errStopped
	default:
		return f.ReadSeeker.Read(b)
	}
}

// deviceName turns a User-Agent into something like "Edge on Windows".
func deviceName(ua string) string {
	var browser, system string
	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/"):
		browser = "Opera"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome/") || strings.Contains(ua, "CriOS/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}
	switch {
	case strings.Contains(ua, "iPhone"):
		system = "iPhone"
	case strings.Contains(ua, "iPad"):
		system = "iPad"
	case strings.Contains(ua, "Android"):
		system = "Android"
	case strings.Contains(ua, "Windows"):
		system = "Windows"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		system = "Mac"
	case strings.Contains(ua, "CrOS"):
		system = "ChromeOS"
	case strings.Contains(ua, "Linux"):
		system = "Linux"
	}
	switch {
	case browser != "" && system != "":
		return browser + " on " + system
	case browser != "":
		return browser
	case system != "":
		return system
	case ua != "":
		name, _, _ := strings.Cut(ua, " ")
		return name
	}
	return "Unknown device"
}

type playbackDTO struct {
	ID         string     `json:"id"`
	Profile    profileDTO `json:"profile"`
	Item       itemDTO    `json:"item"`
	Extra      string     `json:"extra,omitempty"` // the trailer or extra being played, by title
	Device     string     `json:"device"`
	Address    string     `json:"address"`
	Method     string     `json:"method"` // direct, remux or transcode
	PositionMS int64      `json:"position_ms"`
	DurationMS int64      `json:"duration_ms"`
	State      string     `json:"state"` // playing, paused or buffering (a stream is open but no report yet)
	StartedAt  int64      `json:"started_at"`
}

// listPlaying: GET /api/playing is what's being watched right now.
func (s *Server) listPlaying(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := []playbackDTO{}
	for _, p := range s.playing.list() {
		it, err := s.store.GetItem(ctx, p.itemID)
		if err != nil || it == nil {
			continue
		}
		var user store.User
		if u, err := s.store.GetUser(ctx, p.userID); err == nil && u != nil {
			user = *u
		}
		d := playbackDTO{ID: p.id, Profile: toProfile(user), Device: p.device, Address: p.address,
			Method: cmp.Or(p.method, "direct"), PositionMS: p.positionMS, DurationMS: p.durationMS, StartedAt: p.started.Unix()}
		d.Item = toDTO(*it)
		if it.Kind == "episode" {
			if series, err := s.store.GetItem(ctx, it.ParentID); err == nil && series != nil {
				pos, _, _ := s.orderPositions(ctx, p.userID, series)
				d.Item = s.episodeDTO(*it, series, pos)
			}
		}
		if p.extraID != "" {
			if e, err := s.store.GetExtra(ctx, p.extraID); err == nil && e != nil {
				d.Extra = e.Title
				d.DurationMS = e.DurationMS
			}
		}
		if d.DurationMS == 0 {
			if files, err := s.store.ItemFiles(ctx, it.ID); err == nil && len(files) > 0 && p.extraID == "" {
				d.DurationMS = files[0].DurationMS
			}
		}
		switch {
		case p.paused:
			d.State = "paused"
		case p.positionMS == 0 && p.streaming():
			d.State = "buffering"
		default:
			d.State = "playing"
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

// stopPlaying: POST /api/playing/{id}/stop {"message": "..."} stops a playback; the player shows
// the message.
func (s *Server) stopPlaying(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Message string `json:"message"`
	}
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, `send {"message": "..."} or nothing`)
			return
		}
	}
	msg := strings.TrimSpace(body.Message)
	if len(msg) > 300 {
		msg = msg[:300]
	}
	found, sessionID := s.playing.stop(r.PathValue("id"), msg)
	if sessionID != "" && s.transcoder != nil {
		s.transcoder.Stop(sessionID)
	}
	if !found {
		writeError(w, http.StatusNotFound, "that has already finished")
		return
	}
	s.log.Info("stopped a stream", "by", currentUser(r).Name)
	w.WriteHeader(http.StatusNoContent)
}

// serverLog: GET /api/logs?level=info&after=<seq> is the server's recent log lines, oldest first.
func (s *Server) serverLog(w http.ResponseWriter, r *http.Request) {
	level := slog.LevelInfo
	if v := r.URL.Query().Get("level"); v != "" {
		if err := level.UnmarshalText([]byte(v)); err != nil {
			writeError(w, http.StatusBadRequest, "level is debug, info, warn or error")
			return
		}
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if s.logs == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, s.logs.Lines(level, after))
}

// remoteOthers counts the other playbacks going to people away from home (not this request's).
func (n *nowPlaying) remoteOthers(r *http.Request, itemID string) int {
	key := playbackKey(r, itemID, "")
	n.mu.Lock()
	defer n.mu.Unlock()
	count := 0
	for k, p := range n.byKey {
		if k != key && !isHomeAddress(p.address) && time.Since(p.updated) < playingTimeout {
			count++
		}
	}
	return count
}

// isHomeAddress reports whether an IP address is this computer or the home network.
func isHomeAddress(addr string) bool { return auth.IsHomeIP(addr) }

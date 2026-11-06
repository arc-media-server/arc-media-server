// Package watchtogether owns ephemeral shared timelines, never playback streams or media.
// No database, network I/O or goroutines run under its lock. Changes wake bounded SSE readers;
// readers fetch a fresh snapshot rather than accumulating per-client event queues.
package watchtogether

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	MaxRooms        = 20
	MaxParticipants = 10
	InviteLife      = 15 * time.Minute
	MemberLife      = 30 * time.Second
	EmptyLife       = 3 * time.Minute
	EndRetention    = 30 * time.Second
	WaitLife        = 10 * time.Second
	MaxPositionMS   = int64(7 * 24 * 60 * 60 * 1000)
)

var (
	ErrNotFound  = errors.New("room not found")
	ErrLimit     = errors.New("watch together limit reached")
	ErrRate      = errors.New("too many room updates")
	ErrStale     = errors.New("room changed")
	ErrForbidden = errors.New("only the host can end the room or renew its invite")
	ErrInput     = errors.New("invalid room input")
)

type Actor struct{ Session, UserID, Name string }

func (a Actor) Key() string { return a.Session + "|" + a.UserID }

type Member struct {
	ID               string
	Actor            Actor
	Available, Ready bool
	PositionMS       int64
	State            string
	LastSeen         time.Time
	// The member's open room stream, if any: a newer one replaces it (streamStop is closed).
	streamToken            string
	streamStop             chan struct{}
	commandAt, heartbeatAt time.Time
	commands, heartbeats   int
}
type Snapshot struct {
	ID, ItemID, State, HostID, ChangedBy   string
	Revision, Generation, PositionMS, AtMS int64
	Rate                                   float64
	Members                                []Member
	WaitUntilMS                            int64
}
type room struct {
	Snapshot
	members                                              []*Member
	code                                                 string
	inviteUntil, created, emptySince, endedAt, waitUntil time.Time
	playing                                              bool
	ignored                                              map[string]bool
	peak                                                 int
	changed                                              chan struct{}
}
type Manager struct {
	mu      sync.Mutex
	rooms   map[string]*room
	streams map[string]bool
	now     func() time.Time
	log     *slog.Logger
}

func New(log *slog.Logger) *Manager {
	return &Manager{rooms: map[string]*room{}, streams: map[string]bool{}, now: time.Now, log: log}
}
func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func newCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	for i, v := range b {
		b[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(b[:])
}

func (m *Manager) uniqueCode() string {
	for {
		code := newCode()
		found := false
		for _, r := range m.rooms {
			if r.code == code {
				found = true
				break
			}
		}
		if !found {
			return code
		}
	}
}

// NormalizeCode permits the readable XXXX-XXXX spelling without accepting unbounded inputs.
func NormalizeCode(code string) string {
	if len(code) > 16 {
		return ""
	}
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
}
func (m *Manager) changed(r *room) { r.Revision++; close(r.changed); r.changed = make(chan struct{}) }
func position(r *room, now time.Time) int64 {
	p := r.PositionMS
	if r.State == "playing" {
		p += max(int64(0), now.UnixMilli()-r.AtMS)
	}
	return min(p, MaxPositionMS)
}
func (m *Manager) freeze(r *room, now time.Time) {
	r.PositionMS = position(r, now)
	r.AtMS = now.UnixMilli()
}
func (m *Manager) logInfo(message string, r *room, reason string) {
	if m.log != nil {
		m.log.Info(message, "room_id", r.ID, "participants", len(r.members), "peak", r.peak, "duration", m.now().Sub(r.created).Round(time.Second), "reason", reason)
	}
}
func (m *Manager) settle(r *room, now time.Time) {
	if r.State == "ended" {
		return
	}
	blocked := false
	for _, p := range r.members {
		if p.Available && !p.Ready && !r.ignored[p.ID] {
			blocked = true
		}
	}
	target := "paused"
	if r.playing {
		target = "playing"
		if blocked {
			target = "waiting"
		}
	}
	transitionAt := now
	if target == "waiting" && r.State == "waiting" && !r.waitUntil.IsZero() && !now.Before(r.waitUntil) {
		for _, p := range r.members {
			if !p.Ready {
				r.ignored[p.ID] = true
			}
		}
		target = "playing"
		transitionAt = r.waitUntil
	}
	if target != r.State {
		m.freeze(r, transitionAt)
		r.State = target
		if target == "waiting" {
			r.waitUntil = now.Add(WaitLife)
		} else {
			r.waitUntil = time.Time{}
		}
		m.changed(r)
		if m.log != nil {
			m.log.Debug("watch together waiting decision", "room_id", r.ID, "state", target, "participants", len(r.members))
		}
	}
}
func (m *Manager) remove(r *room, key, reason string, now time.Time) {
	for i, p := range r.members {
		if p.Actor.Key() == key {
			r.members = append(r.members[:i], r.members[i+1:]...)
			delete(r.ignored, p.ID)
			if r.HostID == p.ID {
				r.HostID = ""
				if len(r.members) > 0 {
					r.HostID = r.members[0].ID
				}
			}
			if len(r.members) == 0 {
				r.emptySince = now
				m.freeze(r, now)
				r.playing = false
				r.State = "paused"
				r.waitUntil = time.Time{}
			}
			r.ChangedBy = p.ID
			m.changed(r)
			m.settle(r, now)
			m.logInfo("watch together participant left", r, reason)
			return
		}
	}
}
func (m *Manager) end(r *room, reason string, now time.Time) {
	m.freeze(r, now)
	r.State = "ended"
	r.playing = false
	r.endedAt = now
	r.code = ""
	m.changed(r)
	m.logInfo("watch together ended", r, reason)
}

// sweep is lazy when there are no readers. Expired rooms cannot be accessed or consume new-room
// capacity after any API operation. Active SSE readers advance deadlines every 200 ms.
func (m *Manager) sweep(now time.Time) {
	for id, r := range m.rooms {
		if !r.endedAt.IsZero() {
			if now.Sub(r.endedAt) >= EndRetention {
				close(r.changed)
				delete(m.rooms, id)
			}
			continue
		}
		var lastPresence time.Time
		for _, p := range r.members {
			if p.LastSeen.After(lastPresence) {
				lastPresence = p.LastSeen
			}
		}
		allExpired := !lastPresence.IsZero() && now.Sub(lastPresence) >= MemberLife
		if allExpired {
			m.freeze(r, lastPresence.Add(MemberLife))
			r.playing = false
			r.State = "paused"
			r.waitUntil = time.Time{}
		}
		for i := len(r.members) - 1; i >= 0; i-- {
			if now.Sub(r.members[i].LastSeen) >= MemberLife {
				m.remove(r, r.members[i].Actor.Key(), "presence timeout", now)
			}
		}
		if allExpired {
			r.emptySince = lastPresence.Add(MemberLife)
		}
		if len(r.members) == 0 && !r.emptySince.IsZero() && now.Sub(r.emptySince) >= EmptyLife {
			m.end(r, "empty room expired", r.emptySince.Add(EmptyLife))
			if now.Sub(r.endedAt) >= EndRetention {
				close(r.changed)
				delete(m.rooms, id)
			}
			continue
		}
		m.settle(r, now)
	}
}
func (m *Manager) member(r *room, key string) *Member {
	for _, p := range r.members {
		if p.Actor.Key() == key {
			return p
		}
	}
	return nil
}
func (m *Manager) snapshot(r *room) Snapshot {
	out := r.Snapshot
	out.Rate = 1
	out.WaitUntilMS = r.waitUntil.UnixMilli()
	if r.waitUntil.IsZero() {
		out.WaitUntilMS = 0
	}
	out.Members = make([]Member, len(r.members))
	for i, p := range r.members {
		out.Members[i] = *p
	}
	return out
}
func (m *Manager) lookup(id, key string) (*room, *Member, error) {
	r := m.rooms[id]
	if r == nil {
		return nil, nil, ErrNotFound
	}
	p := m.member(r, key)
	if p == nil {
		return nil, nil, ErrNotFound
	}
	return r, p, nil
}
func (m *Manager) Create(a Actor, item string, start int64) (Snapshot, string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	if a.Session == "" || a.UserID == "" || item == "" || start < 0 || start > MaxPositionMS {
		return Snapshot{}, "", time.Time{}, ErrInput
	}
	if len(m.rooms) >= MaxRooms {
		return Snapshot{}, "", time.Time{}, ErrLimit
	}
	for _, r := range m.rooms {
		if r.State != "ended" && m.member(r, a.Key()) != nil {
			return Snapshot{}, "", time.Time{}, ErrLimit
		}
	}
	p := &Member{ID: randomID(), Actor: a, Available: true, LastSeen: now, State: "paused"}
	// uniqueCode avoids ambiguity even in the improbable event of an invite collision.
	r := &room{Snapshot: Snapshot{ID: randomID(), ItemID: item, State: "paused", HostID: p.ID, Revision: 1, Generation: 1, PositionMS: start, AtMS: now.UnixMilli(), Rate: 1}, members: []*Member{p}, code: m.uniqueCode(), inviteUntil: now.Add(InviteLife), created: now, peak: 1, ignored: map[string]bool{}, changed: make(chan struct{})}
	m.rooms[r.ID] = r
	m.logInfo("watch together created", r, "created")
	return m.snapshot(r), r.code, r.inviteUntil, nil
}

// InviteItem reveals only the candidate item internally so the API can check permissions before
// admitting anyone. It never returns a room snapshot to a code holder who lacks media access.
func (m *Manager) InviteItem(code string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	code = NormalizeCode(code)
	for _, r := range m.rooms {
		if code != "" && r.code == code && r.State != "ended" && now.Before(r.inviteUntil) {
			return r.ItemID, nil
		}
	}
	return "", ErrNotFound
}
func (m *Manager) Join(a Actor, code, authorizedItem string) (Snapshot, error) {
	if a.Session == "" || a.UserID == "" {
		return Snapshot{}, ErrInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	code = NormalizeCode(code)
	for _, r := range m.rooms {
		if code == "" || r.code != code || r.State == "ended" || !now.Before(r.inviteUntil) {
			continue
		}
		if r.ItemID != authorizedItem {
			return Snapshot{}, ErrStale
		}
		if p := m.member(r, a.Key()); p != nil {
			p.LastSeen = now
			return m.snapshot(r), nil
		}
		if len(r.members) >= MaxParticipants {
			return Snapshot{}, ErrLimit
		}
		for _, other := range m.rooms {
			if other.State != "ended" && m.member(other, a.Key()) != nil {
				return Snapshot{}, ErrLimit
			}
		}
		p := &Member{ID: randomID(), Actor: a, Available: true, State: "waiting", LastSeen: now}
		r.members = append(r.members, p)
		r.peak = max(r.peak, len(r.members))
		// Joining a room that's already playing doesn't pause everyone: the newcomer catches up
		// to the shared position by themselves, as in Plex, and joins the barrier once ready.
		if r.playing {
			r.ignored[p.ID] = true
		}
		r.emptySince = time.Time{}
		if r.HostID == "" {
			r.HostID = p.ID
		}
		r.ChangedBy = p.ID
		m.changed(r)
		m.settle(r, now)
		m.logInfo("watch together participant joined", r, "joined")
		return m.snapshot(r), nil
	}
	return Snapshot{}, ErrNotFound
}
func (m *Manager) Get(id, key string) (Snapshot, <-chan struct{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep(m.now())
	r, _, err := m.lookup(id, key)
	if err != nil {
		return Snapshot{}, nil, err
	}
	return m.snapshot(r), r.changed, nil
}

// SetAccess is called after fresh profile/media authorization. Unavailable members remain in the
// room but never hold up the ready barrier. Ended-room membership is retained only for its tombstone.
func (m *Manager) SetAccess(id, key, item string, allowed bool) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	r, p, err := m.lookup(id, key)
	if err != nil {
		return Snapshot{}, err
	}
	if item != r.ItemID {
		return m.snapshot(r), ErrStale
	}
	if p.Available != allowed {
		p.Available = allowed
		if !allowed {
			p.Ready = false
		}
		m.changed(r)
		m.settle(r, now)
	}
	return m.snapshot(r), nil
}
func rate(at *time.Time, count *int, now time.Time, maxCount int) bool {
	if now.Sub(*at) >= time.Second {
		*at = now
		*count = 0
	}
	if *count >= maxCount {
		return false
	}
	*count++
	return true
}
func (m *Manager) Command(id, key string, revision int64, action, item string, pos int64, allowed map[string]bool) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	r, p, err := m.lookup(id, key)
	if err != nil {
		return Snapshot{}, err
	}
	if !rate(&p.commandAt, &p.commands, now, 10) {
		return m.snapshot(r), ErrRate
	}
	if revision != r.Revision {
		return m.snapshot(r), ErrStale
	}
	if r.State == "ended" {
		return m.snapshot(r), ErrNotFound
	}
	if pos < 0 || pos > MaxPositionMS {
		return m.snapshot(r), ErrInput
	}
	if action == "end" {
		if p.ID != r.HostID {
			return m.snapshot(r), ErrForbidden
		}
		r.ChangedBy = p.ID
		m.end(r, "host ended", now)
		return m.snapshot(r), nil
	}
	if action != "change_item" && !p.Available {
		return m.snapshot(r), ErrNotFound
	}
	switch action {
	case "play", "pause", "seek", "change_item":
	default:
		return m.snapshot(r), ErrInput
	}
	if action == "change_item" && (item == "" || !allowed[key]) {
		return m.snapshot(r), ErrInput
	}
	m.freeze(r, now)
	switch action {
	case "play":
		r.playing = true
	case "pause":
		r.playing = false
	case "seek":
		r.PositionMS = pos
		r.Generation++
		if r.State == "waiting" {
			r.waitUntil = now.Add(WaitLife)
		}
		r.ignored = map[string]bool{}
		for _, member := range r.members {
			member.Ready = false
		}
	case "change_item":
		r.ItemID = item
		r.PositionMS = pos
		r.Generation++
		if r.State == "waiting" {
			r.waitUntil = now.Add(WaitLife)
		}
		r.ignored = map[string]bool{}
		for _, member := range r.members {
			member.Available = allowed[member.Actor.Key()]
			member.Ready = false
		}
	default:
		return m.snapshot(r), ErrInput
	}
	p.LastSeen = now
	r.ChangedBy = p.ID
	m.changed(r)
	m.settle(r, now)
	if m.log != nil {
		m.log.Debug("watch together command", "room_id", r.ID, "command", action, "revision", r.Revision)
	}
	return m.snapshot(r), nil
}
func (m *Manager) Heartbeat(id, key string, generation, pos int64, state string, ready bool) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	r, p, err := m.lookup(id, key)
	if err != nil {
		return Snapshot{}, err
	}
	if r.State == "ended" {
		return m.snapshot(r), ErrNotFound
	}
	if !rate(&p.heartbeatAt, &p.heartbeats, now, 4) {
		return m.snapshot(r), ErrRate
	}
	if generation != r.Generation {
		return m.snapshot(r), ErrStale
	}
	if pos < 0 || pos > MaxPositionMS || (state != "playing" && state != "paused" && state != "waiting") || (state == "waiting" && ready) {
		return m.snapshot(r), ErrInput
	}
	p.LastSeen = now
	p.PositionMS = pos
	p.State = state
	ready = ready && p.Available
	if p.Ready != ready {
		p.Ready = ready
		if ready {
			delete(r.ignored, p.ID)
		}
		m.changed(r)
		m.settle(r, now)
	}
	return m.snapshot(r), nil
}
func (m *Manager) Touch(id, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep(m.now())
	_, p, err := m.lookup(id, key)
	if err == nil {
		p.LastSeen = m.now()
	}
	return err
}
func (m *Manager) Leave(id, key, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep(m.now())
	r, _, err := m.lookup(id, key)
	if err != nil {
		return err
	}
	m.remove(r, key, reason, m.now())
	return nil
}
func (m *Manager) RenewInvite(id, key string, revision int64) (Snapshot, string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	r, p, err := m.lookup(id, key)
	if err != nil {
		return Snapshot{}, "", time.Time{}, err
	}
	if revision != r.Revision {
		return m.snapshot(r), "", time.Time{}, ErrStale
	}
	if p.ID != r.HostID {
		return m.snapshot(r), "", time.Time{}, ErrForbidden
	}
	if r.State == "ended" {
		return Snapshot{}, "", time.Time{}, ErrNotFound
	}
	if !rate(&p.commandAt, &p.commands, now, 10) {
		return m.snapshot(r), "", time.Time{}, ErrRate
	}
	r.code = m.uniqueCode()
	r.inviteUntil = now.Add(InviteLife)
	m.changed(r)
	return m.snapshot(r), r.code, r.inviteUntil, nil
}

// Stream is one open room stream. Stop closes when a newer stream for the same member replaces
// it: a TV that lost its network or slept reconnects at once, while the server may not notice
// the old connection is dead for minutes (writes to a half-open connection still succeed).
type Stream struct {
	MemberID, Token string
	Stop            <-chan struct{}
}

func (m *Manager) OpenStream(id, key string) (Stream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep(m.now())
	_, p, err := m.lookup(id, key)
	if err != nil {
		return Stream{}, err
	}
	replacing := p.streamToken != ""
	if !replacing && len(m.streams) >= MaxRooms*MaxParticipants {
		return Stream{}, ErrLimit
	}
	if replacing {
		close(p.streamStop)
	}
	token := randomID()
	p.streamToken, p.streamStop = token, make(chan struct{})
	m.streams[token] = true
	return Stream{MemberID: p.ID, Token: token, Stop: p.streamStop}, nil
}
func (m *Manager) CloseStream(id, key, token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.streams, token)
	if r := m.rooms[id]; r != nil {
		if p := m.member(r, key); p != nil && p.streamToken == token {
			p.streamToken, p.streamStop = "", nil
		}
	}
}

// PlayingInfo exposes only correlation/count to the existing admin Playing DTO. No room list API.
func (m *Manager) PlayingInfo(session, user, item string) (string, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep(m.now())
	key := (Actor{Session: session, UserID: user}).Key()
	for _, r := range m.rooms {
		if r.State != "ended" && r.ItemID == item && m.member(r, key) != nil {
			return r.ID, len(r.members)
		}
	}
	return "", 0
}

// Current recovers this exact device/profile's own active room after reload or a lost create
// response. This is not room discovery: callers cannot enumerate any other membership.
func (m *Manager) Current(key string) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep(m.now())
	for _, r := range m.rooms {
		if r.State != "ended" && m.member(r, key) != nil {
			return m.snapshot(r), nil
		}
	}
	return Snapshot{}, ErrNotFound
}

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
	// InvitedLife is how long a room with invited people waits while nobody is in it, so the
	// people invited have time to see it on Home and join.
	InvitedLife = time.Hour
	// MaxHosted bounds the rooms one profile may have waiting with invitations.
	MaxHosted     = 3
	EndRetention  = 30 * time.Second
	WaitLife      = 10 * time.Second
	MaxPositionMS = int64(7 * 24 * 60 * 60 * 1000)
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

// Invitee is someone the host invited by profile, rather than by code.
type Invitee struct{ UserID, Name string }

type Snapshot struct {
	ID, ItemID, State, HostID, ChangedBy string
	// The profile that created the room, and its name: it's host again whenever it rejoins.
	HostUserID, HostName string
	// People invited who aren't in the room now (in invitation order).
	Invited                                []Invitee
	CreatedMS                              int64
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
	// invited holds every profile invited by name (user ID -> name), joined or not.
	invited     map[string]string
	inviteOrder []string
}
type Manager struct {
	mu      sync.Mutex
	rooms   map[string]*room
	streams map[string]bool
	now     func() time.Time
	log     *slog.Logger
	// Notify, when set, hears which profiles' room lists changed: a room was made, people were
	// invited, someone joined, left or declined, or it ended. It runs under the manager's lock,
	// so it must be quick and never call back into the manager.
	Notify func(userIDs []string)
}

// notify tells everyone who can see the room in their list that it changed.
func (m *Manager) notify(r *room) {
	if m.Notify == nil {
		return
	}
	seen := map[string]bool{}
	ids := []string{}
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	add(r.HostUserID)
	for _, id := range r.inviteOrder {
		add(id)
	}
	for _, p := range r.members {
		add(p.Actor.UserID)
	}
	m.Notify(ids)
}

// emptyLife is how long the room may stay with nobody in it.
func emptyLife(r *room) time.Duration {
	if len(r.invited) > 0 {
		return InvitedLife
	}
	return EmptyLife
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
			m.notify(r)
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
	m.notify(r)
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
		if life := emptyLife(r); len(r.members) == 0 && !r.emptySince.IsZero() && now.Sub(r.emptySince) >= life {
			m.end(r, "empty room expired", r.emptySince.Add(life))
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
	present := map[string]bool{}
	for i, p := range r.members {
		out.Members[i] = *p
		present[p.Actor.UserID] = true
	}
	out.Invited = nil
	for _, id := range r.inviteOrder {
		if name, ok := r.invited[id]; ok && !present[id] {
			out.Invited = append(out.Invited, Invitee{UserID: id, Name: name})
		}
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
	r := m.newRoom(a, item, start, now)
	r.HostID = p.ID
	r.members = []*Member{p}
	r.peak = 1
	m.rooms[r.ID] = r
	m.logInfo("watch together created", r, "created")
	return m.snapshot(r), r.code, r.inviteUntil, nil
}

func (m *Manager) newRoom(a Actor, item string, start int64, now time.Time) *room {
	// uniqueCode avoids ambiguity even in the improbable event of an invite collision.
	return &room{Snapshot: Snapshot{ID: randomID(), ItemID: item, State: "paused", HostUserID: a.UserID, HostName: a.Name, CreatedMS: now.UnixMilli(), Revision: 1, Generation: 1, PositionMS: start, AtMS: now.UnixMilli(), Rate: 1}, code: m.uniqueCode(), inviteUntil: now.Add(InviteLife), created: now, ignored: map[string]bool{}, changed: make(chan struct{}), invited: map[string]string{}}
}

// CreateInvited makes a room for people chosen by profile: nobody is in it yet.
// It waits (InvitedLife) for them and for its host to join from their room lists; whoever made
// it is host when they join. The code works as for any room.
func (m *Manager) CreateInvited(a Actor, item string, start int64, people []Invitee) (Snapshot, string, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	if a.Session == "" || a.UserID == "" || item == "" || start < 0 || start > MaxPositionMS || len(people) == 0 || len(people) >= MaxParticipants {
		return Snapshot{}, "", time.Time{}, ErrInput
	}
	if len(m.rooms) >= MaxRooms {
		return Snapshot{}, "", time.Time{}, ErrLimit
	}
	hosted := 0
	for _, r := range m.rooms {
		if r.State != "ended" && r.HostUserID == a.UserID && len(r.invited) > 0 {
			hosted++
		}
	}
	if hosted >= MaxHosted {
		return Snapshot{}, "", time.Time{}, ErrLimit
	}
	r := m.newRoom(a, item, start, now)
	r.emptySince = now
	for _, p := range people {
		if p.UserID == "" || p.UserID == a.UserID {
			return Snapshot{}, "", time.Time{}, ErrInput
		}
		if _, dup := r.invited[p.UserID]; !dup {
			r.invited[p.UserID] = p.Name
			r.inviteOrder = append(r.inviteOrder, p.UserID)
		}
	}
	m.rooms[r.ID] = r
	m.notify(r)
	m.logInfo("watch together created", r, "created with invitations")
	return m.snapshot(r), r.code, r.inviteUntil, nil
}

// Invite adds people to a room (host only); people already invited are skipped.
func (m *Manager) Invite(id, key string, people []Invitee) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	r, p, err := m.lookup(id, key)
	if err != nil {
		return Snapshot{}, err
	}
	if r.State == "ended" {
		return Snapshot{}, ErrNotFound
	}
	if p.ID != r.HostID {
		return m.snapshot(r), ErrForbidden
	}
	if !rate(&p.commandAt, &p.commands, now, 10) {
		return m.snapshot(r), ErrRate
	}
	added := false
	for _, person := range people {
		if person.UserID == "" || person.UserID == p.Actor.UserID {
			return m.snapshot(r), ErrInput
		}
		if _, dup := r.invited[person.UserID]; dup {
			continue
		}
		if len(r.invited) >= MaxParticipants-1 {
			return m.snapshot(r), ErrLimit
		}
		r.invited[person.UserID] = person.Name
		r.inviteOrder = append(r.inviteOrder, person.UserID)
		added = true
	}
	if added {
		r.ChangedBy = p.ID
		m.changed(r)
		m.notify(r)
	}
	return m.snapshot(r), nil
}

// visibleTo says whether the profile sees the room in its list: it made it, was invited, or is in it.
func (m *Manager) visibleTo(r *room, userID string) bool {
	if r.State == "ended" || userID == "" {
		return false
	}
	if r.HostUserID == userID {
		return true
	}
	if _, ok := r.invited[userID]; ok {
		return true
	}
	for _, p := range r.members {
		if p.Actor.UserID == userID {
			return true
		}
	}
	return false
}

// Rooms lists the active rooms a profile made, was invited to or is in, newest first. The caller
// still checks the profile may play each room's title before showing it.
func (m *Manager) Rooms(userID string) []Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep(m.now())
	out := []Snapshot{}
	for _, r := range m.rooms {
		if m.visibleTo(r, userID) {
			out = append(out, m.snapshot(r))
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].CreatedMS > out[j-1].CreatedMS; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// RoomItem is the title of a room the profile may see in its list, so the API can check access
// before letting it in.
func (m *Manager) RoomItem(id, userID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweep(m.now())
	r := m.rooms[id]
	if r == nil || !m.visibleTo(r, userID) {
		return "", ErrNotFound
	}
	return r.ItemID, nil
}

// Accept joins a room from the profile's list (no code needed), if it may still see the room.
func (m *Manager) Accept(a Actor, id, authorizedItem string) (Snapshot, error) {
	if a.Session == "" || a.UserID == "" {
		return Snapshot{}, ErrInput
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	r := m.rooms[id]
	if r == nil || !m.visibleTo(r, a.UserID) {
		return Snapshot{}, ErrNotFound
	}
	if r.ItemID != authorizedItem {
		return Snapshot{}, ErrStale
	}
	return m.admit(r, a, now)
}

// Dismiss takes a room off a profile's list: someone invited declines; its host cancels it for
// everyone (as ending it does).
func (m *Manager) Dismiss(id, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	m.sweep(now)
	r := m.rooms[id]
	if r == nil || !m.visibleTo(r, userID) {
		return ErrNotFound
	}
	if r.HostUserID == userID {
		m.end(r, "cancelled by host", now)
		return nil
	}
	if _, ok := r.invited[userID]; !ok {
		return ErrNotFound
	}
	delete(r.invited, userID)
	for i, id := range r.inviteOrder {
		if id == userID {
			r.inviteOrder = append(r.inviteOrder[:i], r.inviteOrder[i+1:]...)
			break
		}
	}
	m.changed(r)
	m.notify(r)
	// The decliner is no longer listed, so tell them too.
	if m.Notify != nil {
		m.Notify([]string{userID})
	}
	m.logInfo("watch together invitation declined", r, "declined")
	return nil
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
		return m.admit(r, a, now)
	}
	return Snapshot{}, ErrNotFound
}

// admit lets a profile on a device into a room (or finds it already there).
func (m *Manager) admit(r *room, a Actor, now time.Time) (Snapshot, error) {
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
	// Whoever made the room is its host when they come (back). A room made by invitation has
	// no host until then: the people invited wait for its creator to start it.
	if (r.HostID == "" && len(r.invited) == 0) || (a.UserID == r.HostUserID && m.hostUser(r) != r.HostUserID) {
		r.HostID = p.ID
	}
	r.ChangedBy = p.ID
	m.changed(r)
	m.settle(r, now)
	m.notify(r)
	m.logInfo("watch together participant joined", r, "joined")
	return m.snapshot(r), nil
}

// hostUser is the profile of the room's current host, if any.
func (m *Manager) hostUser(r *room) string {
	for _, p := range r.members {
		if p.ID == r.HostID {
			return p.Actor.UserID
		}
	}
	return ""
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

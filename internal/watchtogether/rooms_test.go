package watchtogether

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Manager, *time.Time, Actor) {
	t.Helper()
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	m := New(slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.now = func() time.Time { return now }
	return m, &now, Actor{Session: "device-a", UserID: "profile", Name: "Host"}
}
func get(t *testing.T, m *Manager, id string, a Actor) Snapshot {
	t.Helper()
	s, _, err := m.Get(id, a.Key())
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func command(t *testing.T, m *Manager, s Snapshot, a Actor, action, item string, pos int64, allowed map[string]bool) Snapshot {
	t.Helper()
	next, err := m.Command(s.ID, a.Key(), s.Revision, action, item, pos, allowed)
	if err != nil {
		t.Fatal(err)
	}
	return next
}
func ready(t *testing.T, m *Manager, s Snapshot, a Actor, v bool) Snapshot {
	t.Helper()
	state := "waiting"
	if v {
		state = "paused"
	}
	next, err := m.Heartbeat(s.ID, a.Key(), s.Generation, 0, state, v)
	if err != nil {
		t.Fatal(err)
	}
	return next
}
func TestTimelineWaitingTimeoutAndGenerations(t *testing.T) {
	m, now, a := fixture(t)
	s, code, _, err := m.Create(a, "movie", 1000)
	if err != nil {
		t.Fatal(err)
	}
	b := Actor{Session: "device-b", UserID: "profile", Name: "Second device"}
	s, err = m.Join(b, code, "movie")
	if err != nil {
		t.Fatal(err)
	}
	s = ready(t, m, s, a, true)
	s = ready(t, m, s, b, true)
	if s.Started || s.PositionMS != 1000 {
		t.Fatal("a room made part-way through hasn't started", s)
	}
	s = command(t, m, s, a, "play", "", 0, nil)
	if s.State != "playing" || !s.Started {
		t.Fatal(s)
	}
	*now = now.Add(2 * time.Second)
	s = ready(t, m, get(t, m, s.ID, b), b, false)
	if s.State != "waiting" || s.PositionMS != 3000 {
		t.Fatal(s)
	}
	*now = now.Add(9 * time.Second)
	s = get(t, m, s.ID, a)
	if s.State != "waiting" {
		t.Fatal(s)
	}
	old := s.Revision
	*now = now.Add(time.Second)
	s = get(t, m, s.ID, a)
	if s.State != "playing" || s.PositionMS != 3000 {
		t.Fatal("timeout", s)
	}
	if current, err := m.Command(s.ID, a.Key(), old, "pause", "", 0, nil); !errors.Is(err, ErrStale) || current.Revision != s.Revision {
		t.Fatal("stale command", err)
	}
	*now = now.Add(time.Second)
	s = ready(t, m, s, b, false)
	if s.State != "playing" {
		t.Fatal("slow client restarted barrier", s)
	}
	s = command(t, m, s, a, "seek", "", 50000, nil)
	if s.State != "waiting" || s.Generation != 2 || s.PositionMS != 50000 {
		t.Fatal(s)
	}
	if _, err = m.Heartbeat(s.ID, b.Key(), 1, 0, "playing", true); !errors.Is(err, ErrStale) {
		t.Fatal("old seek ack accepted")
	}
	s = ready(t, m, s, a, true)
	s = ready(t, m, s, b, true)
	if s.State != "playing" {
		t.Fatal(s)
	}
	s = command(t, m, s, a, "change_item", "episode", 0, map[string]bool{a.Key(): true, b.Key(): false})
	if s.Generation != 3 || s.Members[1].Available {
		t.Fatal(s)
	}
	s = ready(t, m, s, a, true)
	if s.State != "playing" {
		t.Fatal("restricted viewer blocks readiness", s)
	}
}
func TestRoomMembershipHostTransferAndStreams(t *testing.T) {
	m, _, a := fixture(t)
	s, code, _, _ := m.Create(a, "movie", 0)
	b := Actor{Session: "device-b", UserID: a.UserID}
	s, err := m.Join(b, code, "movie")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Members) != 2 || s.Members[0].ID == s.Members[1].ID {
		t.Fatal("device identity collapsed")
	}
	if _, _, err = m.Get(s.ID, "stranger"); !errors.Is(err, ErrNotFound) {
		t.Fatal("nonmember access")
	}
	member, err := m.OpenStream(s.ID, a.Key())
	if err != nil {
		t.Fatal(err)
	}
	// A reconnect (the old connection may be half-open for minutes) replaces the old stream.
	reconnect, err := m.OpenStream(s.ID, a.Key())
	if err != nil {
		t.Fatal("a reconnect was refused while the old stream lingered", err)
	}
	select {
	case <-member.Stop:
	default:
		t.Fatal("the replaced stream wasn't told to stop")
	}
	m.CloseStream(s.ID, a.Key(), member.Token) // the old handler exits
	select {
	case <-reconnect.Stop:
		t.Fatal("closing the old stream stopped the new one")
	default:
	}
	m.CloseStream(s.ID, a.Key(), reconnect.Token)
	if err = m.Leave(s.ID, a.Key(), "left"); err != nil {
		t.Fatal(err)
	}
	s = get(t, m, s.ID, b)
	if s.HostID != s.Members[0].ID {
		t.Fatal("host not passed")
	}
	s, err = m.Join(a, code, "movie")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := m.OpenStream(s.ID, a.Key())
	if err != nil {
		t.Fatal(err)
	}
	if fresh.MemberID == member.MemberID {
		t.Fatal("rejoining reused the old membership")
	}
	m.CloseStream(s.ID, a.Key(), fresh.Token)
	if _, err = m.Command(s.ID, a.Key(), s.Revision, "end", "", 0, nil); !errors.Is(err, ErrForbidden) {
		t.Fatal("nonhost ended room")
	}
	s = command(t, m, s, b, "end", "", 0, nil)
	if s.State != "ended" {
		t.Fatal(s)
	}
	if _, err = m.InviteItem(code); !errors.Is(err, ErrNotFound) {
		t.Fatal("ended invite works")
	}
}
func TestExpiryInvitesLimitsAndRates(t *testing.T) {
	m, now, a := fixture(t)
	s, code, expiry, _ := m.Create(a, "movie", 0)
	if expiry.Sub(*now) != InviteLife {
		t.Fatal("invite lifetime")
	}
	_, newCode, _, err := m.RenewInvite(s.ID, a.Key(), s.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.InviteItem(code); !errors.Is(err, ErrNotFound) {
		t.Fatal("old invite valid")
	}
	if item, err := m.InviteItem(newCode[:4] + "-" + newCode[4:]); err != nil || item != "movie" {
		t.Fatal("readable code", err)
	}
	for i := 1; i < MaxParticipants; i++ {
		_, err = m.Join(Actor{Session: fmt.Sprint(i), UserID: "friend"}, newCode, "movie")
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err = m.Join(Actor{Session: "overflow", UserID: "friend"}, newCode, "movie"); !errors.Is(err, ErrLimit) {
		t.Fatal("member cap")
	}
	s = get(t, m, s.ID, a)
	for range 10 {
		_, err = m.Command(s.ID, a.Key(), 0, "pause", "", 0, nil)
		if !errors.Is(err, ErrStale) && !errors.Is(err, ErrRate) {
			t.Fatal(err)
		}
	}
	if _, err = m.Command(s.ID, a.Key(), s.Revision, "play", "", 0, nil); !errors.Is(err, ErrRate) {
		t.Fatal("command cap")
	}
	for range 4 {
		_, err = m.Heartbeat(s.ID, a.Key(), s.Generation, 0, "paused", false)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err = m.Heartbeat(s.ID, a.Key(), s.Generation, 0, "paused", false); !errors.Is(err, ErrRate) {
		t.Fatal("heartbeat cap")
	}
	*now = now.Add(InviteLife + time.Minute)
	if _, err = m.InviteItem(newCode); !errors.Is(err, ErrNotFound) {
		t.Fatal("expired code accepted")
	}
	if len(m.rooms) != 0 {
		t.Fatal("unobserved empty expiry did not free room quota")
	}
	for i := range MaxRooms {
		_, _, _, err = m.Create(Actor{Session: fmt.Sprint(i), UserID: "profile"}, "movie", 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, _, _, err = m.Create(Actor{Session: "overflow", UserID: "profile"}, "movie", 0); !errors.Is(err, ErrLimit) {
		t.Fatal("room cap")
	}
}
func TestConcurrentCommandsHaveOneWinnerAndWakeReaders(t *testing.T) {
	m := New(nil)
	a := Actor{Session: "session", UserID: "user"}
	s, _, _, _ := m.Create(a, "movie", 0)
	_, changed, _ := m.Get(s.ID, a.Key())
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for range 8 {
		wg.Go(func() {
			_, err := m.Command(s.ID, a.Key(), s.Revision, "play", "", 0, nil)
			if err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			} else if !errors.Is(err, ErrStale) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if wins != 1 {
		t.Fatal("lost revision isolation", wins)
	}
	select {
	case <-changed:
	default:
		t.Fatal("reader not woken")
	}
	snap := get(t, m, s.ID, a)
	snap.Members[0].Actor.Name = "tampered"
	if get(t, m, s.ID, a).Members[0].Actor.Name == "tampered" {
		t.Fatal("mutable snapshot alias")
	}
}

func TestDetachedStreamsStayGloballyBounded(t *testing.T) {
	m, _, a := fixture(t)
	s, code, _, _ := m.Create(a, "movie", 0)
	members := []Stream{}
	for range MaxRooms * MaxParticipants {
		stream, err := m.OpenStream(s.ID, a.Key())
		if err != nil {
			t.Fatal(err)
		}
		members = append(members, stream)
		if err = m.Leave(s.ID, a.Key(), "left"); err != nil {
			t.Fatal(err)
		}
		s, err = m.Join(a, code, "movie")
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.OpenStream(s.ID, a.Key()); !errors.Is(err, ErrLimit) {
		t.Fatal("departed slow streams bypassed global cap")
	}
	m.CloseStream(s.ID, a.Key(), members[0].Token)
	if _, err := m.OpenStream(s.ID, a.Key()); err != nil {
		t.Fatal("global slot not released", err)
	}
}
func TestWaitingResumesAtDeadlineWhenReadIsLate(t *testing.T) {
	m, now, a := fixture(t)
	s, _, _, _ := m.Create(a, "movie", 0)
	s = command(t, m, s, a, "play", "", 0, nil)
	deadline := s.WaitUntilMS
	*now = now.Add(WaitLife + 5*time.Second)
	s = get(t, m, s.ID, a)
	if s.State != "playing" || s.AtMS != deadline {
		t.Fatal("late read moved resume anchor", s)
	}
}

// Someone joining a room that's already playing catches up by themselves (as in Plex); they don't
// pause everyone. Once ready they're part of the barrier again, so their later buffering waits.
func TestLateJoinerDoesNotPauseAPlayingRoom(t *testing.T) {
	m, _, a := fixture(t)
	s, code, _, err := m.Create(a, "movie", 0)
	if err != nil {
		t.Fatal(err)
	}
	if s, err = m.Heartbeat(s.ID, a.Key(), s.Generation, 0, "paused", true); err != nil {
		t.Fatal(err)
	}
	s = command(t, m, s, a, "play", "", 0, nil)
	if s.State != "playing" {
		t.Fatalf("host ready, room should play: %s", s.State)
	}
	late := Actor{Session: "device-b", UserID: "friend", Name: "Late"}
	s, err = m.Join(late, code, "movie")
	if err != nil {
		t.Fatal(err)
	}
	if s.State != "playing" {
		t.Fatalf("a late joiner paused the room: %s", s.State)
	}
	if s, err = m.Heartbeat(s.ID, late.Key(), s.Generation, 1000, "playing", true); err != nil {
		t.Fatal(err)
	}
	if s, err = m.Heartbeat(s.ID, late.Key(), s.Generation, 1000, "waiting", false); err != nil {
		t.Fatal(err)
	}
	if s.State != "waiting" {
		t.Fatalf("once ready, the late joiner's buffering should hold the room: %s", s.State)
	}
}

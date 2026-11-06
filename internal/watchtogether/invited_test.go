package watchtogether

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestInvitedRoomsListJoinDeclineAndExpire(t *testing.T) {
	m, now, host := fixture(t)
	var notified [][]string
	m.Notify = func(ids []string) { notified = append(notified, slices.Clone(ids)) }
	guest := Actor{Session: "device-b", UserID: "guest", Name: "Guest"}
	other := Actor{Session: "device-c", UserID: "other", Name: "Other"}
	room, code, _, err := m.CreateInvited(host, "movie", 0, []Invitee{{UserID: "guest", Name: "Guest"}, {UserID: "other", Name: "Other"}})
	if err != nil || code == "" {
		t.Fatal(err)
	}
	if len(room.Members) != 0 || room.HostID != "" || room.HostUserID != "profile" || len(room.Invited) != 2 {
		t.Fatalf("a room made by invitation starts empty: %+v", room)
	}
	if len(notified) != 1 || !slices.Equal(notified[0], []string{"profile", "guest", "other"}) {
		t.Fatal("host and invitees not told", notified)
	}
	for _, user := range []string{"profile", "guest", "other"} {
		if rooms := m.Rooms(user); len(rooms) != 1 || rooms[0].ID != room.ID {
			t.Fatal("room missing from list of", user)
		}
	}
	if len(m.Rooms("stranger")) != 0 {
		t.Fatal("a stranger sees the room")
	}
	if _, err := m.Accept(Actor{Session: "x", UserID: "stranger"}, room.ID, "movie"); !errors.Is(err, ErrNotFound) {
		t.Fatal("a stranger joined without a code", err)
	}
	if _, err := m.Accept(guest, room.ID, "other-movie"); !errors.Is(err, ErrStale) {
		t.Fatal("joined with a title it wasn't checked for", err)
	}

	// Empty rooms with invitations outlast EmptyLife, so people have time to see them.
	*now = now.Add(EmptyLife + time.Minute)
	joined, err := m.Accept(guest, room.ID, "movie")
	if err != nil || len(joined.Members) != 1 || joined.HostID != "" {
		t.Fatal("someone invited became host before the creator came", err, joined)
	}
	if len(joined.Invited) != 1 || joined.Invited[0].UserID != "other" {
		t.Fatal("joined people still listed as invited", joined.Invited)
	}
	// Whoever made the room is host again when they come.
	withHost, err := m.Accept(host, room.ID, "movie")
	if err != nil || withHost.HostID != withHost.Members[1].ID {
		t.Fatal("the creator isn't host on joining", err, withHost)
	}

	// Declining takes the room off that profile's list only.
	if err := m.Dismiss(room.ID, "other"); err != nil {
		t.Fatal(err)
	}
	if len(m.Rooms("other")) != 0 || len(m.Rooms("guest")) != 1 {
		t.Fatal("decline")
	}
	if !slices.Contains(notified[len(notified)-1], "other") {
		t.Fatal("the decliner's list wasn't refreshed")
	}
	if err := m.Dismiss(room.ID, "other"); !errors.Is(err, ErrNotFound) {
		t.Fatal("declined twice")
	}
	if _, err := m.Accept(other, room.ID, "movie"); !errors.Is(err, ErrNotFound) {
		t.Fatal("joined after declining")
	}

	// The host cancelling ends it for everyone.
	if err := m.Dismiss(room.ID, "profile"); err != nil {
		t.Fatal(err)
	}
	if len(m.Rooms("guest")) != 0 || get(t, m, room.ID, guest).State != "ended" {
		t.Fatal("cancel")
	}
}

func TestInvitedRoomsExpireAndAreBounded(t *testing.T) {
	m, now, host := fixture(t)
	people := []Invitee{{UserID: "guest", Name: "Guest"}}
	room, _, _, err := m.CreateInvited(host, "movie", 0, people)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(InvitedLife - time.Second)
	if len(m.Rooms("guest")) != 1 {
		t.Fatal("expired early")
	}
	*now = now.Add(2 * time.Second)
	if len(m.Rooms("guest")) != 0 {
		t.Fatal("an unvisited room lingers", room.ID)
	}
	if _, _, _, err := m.CreateInvited(host, "movie", 0, []Invitee{{UserID: "profile", Name: "Self"}}); !errors.Is(err, ErrInput) {
		t.Fatal("invited themselves")
	}
	if _, _, _, err := m.CreateInvited(host, "movie", 0, nil); !errors.Is(err, ErrInput) {
		t.Fatal("no one invited")
	}
	for i := 0; i < MaxHosted; i++ {
		if _, _, _, err := m.CreateInvited(host, "movie", 0, people); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, _, err := m.CreateInvited(host, "movie", 0, people); !errors.Is(err, ErrLimit) {
		t.Fatal("unbounded waiting rooms per profile")
	}
}

func TestInviteMoreFromInsideTheRoom(t *testing.T) {
	m, _, host := fixture(t)
	room, _, _, err := m.Create(host, "movie", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Rooms("profile")) != 1 {
		t.Fatal("a code room isn't listed for its host")
	}
	guest := Actor{Session: "device-b", UserID: "guest", Name: "Guest"}
	next, err := m.Invite(room.ID, host.Key(), []Invitee{{UserID: "guest", Name: "Guest"}, {UserID: "guest", Name: "Guest"}})
	if err != nil || len(next.Invited) != 1 {
		t.Fatal(err, next.Invited)
	}
	joined, err := m.Accept(guest, room.ID, "movie")
	if err != nil || joined.HostID != room.HostID {
		t.Fatal("an invitee took over as host", err)
	}
	if _, err := m.Invite(room.ID, guest.Key(), []Invitee{{UserID: "x", Name: "X"}}); !errors.Is(err, ErrForbidden) {
		t.Fatal("a guest invited people", err)
	}
}

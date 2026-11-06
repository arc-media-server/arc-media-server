package api

import (
	"ams/internal/events"
	"ams/internal/store"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func TestWatchTogetherInvitationsPeopleListJoinAndDecline(t *testing.T) {
	f := newWatchFixture(t)
	hub := events.New()
	f.s.events = hub
	f.s.wireWatchNotify()

	// Household profiles may invite everyone else; a friend only the household's admins.
	var people struct {
		People []watchPersonDTO `json:"people"`
	}
	w := f.request("GET", "/watch-together/people?item_id="+f.movie, "host", "")
	if json.Unmarshal(w.Body.Bytes(), &people); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	names := map[string]watchPersonDTO{}
	for _, p := range people.People {
		names[p.Name] = p
	}
	if _, self := names[f.host.Name]; self || len(names) != 3 || !names["Friend"].Friend || !names["Child"].CanWatch {
		t.Fatal("household people list", people.People)
	}
	w = f.request("GET", "/watch-together/people?item_id="+f.adult, "host", "")
	people.People = nil
	json.Unmarshal(w.Body.Bytes(), &people)
	for _, p := range people.People {
		if p.Name == "Child" && p.CanWatch {
			t.Fatal("a restricted profile shown as able to watch")
		}
	}
	people.People = nil
	json.Unmarshal(f.request("GET", "/watch-together/people", "friend", "").Body.Bytes(), &people)
	for _, p := range people.People {
		if p.Name == "Child" || p.Name == "Friend" {
			t.Fatal("a friend sees the household's profiles", people.People)
		}
	}
	if len(people.People) == 0 {
		t.Fatal("a friend can invite nobody")
	}

	// Inviting someone who can't watch the title, or a stranger, is refused.
	for _, ids := range [][]string{{f.kid.ID}, {"nobody"}} {
		body, _ := json.Marshal(map[string]any{"item_id": f.adult, "profile_ids": ids})
		if w := f.request("POST", "/watch-together/rooms", "host", string(body)); w.Code != 400 {
			t.Fatal("bad invitation accepted", ids, w.Code, w.Body.String())
		}
	}

	_, cursor, _ := hub.Read("")
	body := fmt.Sprintf(`{"item_id":"%s","profile_ids":["%s","%s"]}`, f.movie, f.kid.ID, f.friend.ID)
	w = f.request("POST", "/watch-together/rooms", "host", body)
	var created watchCreatedDTO
	if json.Unmarshal(w.Body.Bytes(), &created); w.Code != 201 || created.InviteCode == "" || !created.Room.Hosting || len(created.Room.Participants) != 0 || len(created.Room.Invited) != 2 {
		t.Fatal("create", w.Code, w.Body.String())
	}
	batch, _, _ := hub.Read(cursor)
	told := []string{}
	for _, e := range batch {
		if e.Topic == "watch_together" {
			told = append(told, e.UserID)
		}
	}
	if !slices.Contains(told, f.kid.ID) || !slices.Contains(told, f.friend.ID) || !slices.Contains(told, f.host.ID) {
		t.Fatal("invitees' Home rows not refreshed", told)
	}

	list := func(token string) []watchListedDTO {
		t.Helper()
		var out struct {
			Rooms []watchListedDTO `json:"rooms"`
		}
		w := f.request("GET", "/watch-together/rooms", token, "")
		if json.Unmarshal(w.Body.Bytes(), &out); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		return out.Rooms
	}
	if rooms := list("child"); len(rooms) != 1 || rooms[0].Item.ID != f.movie || rooms[0].HostName != f.host.Name || rooms[0].Hosting || rooms[0].Joined {
		t.Fatal("invitee's list", rooms)
	}
	if rooms := list("admin"); len(rooms) != 0 {
		t.Fatal("someone not invited sees the room", rooms)
	}
	if w := f.request("POST", "/watch-together/rooms/"+created.Room.ID+"/join", "admin", ""); w.Code != 404 {
		t.Fatal("joined without an invitation", w.Code)
	}

	// An invitee joins from the list; the host joining later is host.
	room := decodeWatchRoom(t, f.request("POST", "/watch-together/rooms/"+created.Room.ID+"/join", "child", ""))
	if !room.Available || len(room.Participants) != 1 || room.SelfID == "" || len(room.Invited) != 1 || room.HostID != "" || room.HostName != f.host.Name {
		t.Fatal("join", room)
	}
	room = decodeWatchRoom(t, f.request("POST", "/watch-together/rooms/"+created.Room.ID+"/join", "host", ""))
	if room.HostID != room.SelfID {
		t.Fatal("the creator isn't host", room)
	}
	if rooms := list("child"); len(rooms) != 1 || !rooms[0].Joined || len(rooms[0].Participants) != 2 {
		t.Fatal("joined room in list", rooms)
	}

	// A title the invitee may no longer play drops out of their list.
	age := 1
	f.s.store.UpdateUser(t.Context(), f.friend.ID, store.UserUpdate{MaxAge: &age})
	if rooms := list("friend"); len(rooms) != 0 {
		t.Fatal("restricted title listed", rooms)
	}
	if w := f.request("POST", "/watch-together/rooms/"+created.Room.ID+"/join", "friend", ""); w.Code != 404 {
		t.Fatal("restricted join", w.Code)
	}
	f.s.store.UpdateUser(t.Context(), f.friend.ID, store.UserUpdate{MaxAge: new(int)})

	// The friend declines; then the host invites them again from inside the room.
	if w := f.request("POST", "/watch-together/rooms/"+created.Room.ID+"/dismiss", "friend", ""); w.Code != 204 {
		t.Fatal("decline", w.Code)
	}
	if len(list("friend")) != 0 {
		t.Fatal("declined room still listed")
	}
	room = decodeWatchRoom(t, f.request("POST", "/watch-together/"+created.Room.ID+"/people", "host", `{"profile_ids":["`+f.friend.ID+`"]}`))
	if len(room.Invited) != 1 || room.Invited[0].Name != "Friend" {
		t.Fatal("invite more", room.Invited)
	}
	if w := f.request("POST", "/watch-together/"+created.Room.ID+"/people", "child", `{"profile_ids":["`+f.admin.ID+`"]}`); w.Code != 403 {
		t.Fatal("a guest invited people", w.Code)
	}

	// The host cancelling ends the room for everyone.
	if w := f.request("POST", "/watch-together/rooms/"+created.Room.ID+"/dismiss", "host", ""); w.Code != 204 {
		t.Fatal("cancel", w.Code)
	}
	if len(list("child")) != 0 || len(list("host")) != 0 {
		t.Fatal("cancelled room listed")
	}
}

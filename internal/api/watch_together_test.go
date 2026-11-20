package api

import (
	"ams/internal/auth"
	"ams/internal/store"
	"ams/internal/watchtogether"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type watchFixture struct {
	s                        *Server
	manager                  *watchtogether.Manager
	handler                  http.Handler
	host, friend, kid, admin *store.User
	movie, adult, labelled   string
	lib                      int64
}

func newWatchFixture(t *testing.T) *watchFixture {
	t.Helper()
	s, u := securityServer(t)
	ctx := t.Context()
	f := &watchFixture{s: s, host: u, manager: watchtogether.New(s.log)}
	var err error
	f.friend, err = s.store.CreateUser(ctx, "Friend", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetInvite(ctx, f.friend.ID, "fixture-invite", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	f.kid, err = s.store.CreateUser(ctx, "Child", false, "teal", "", "")
	if err != nil {
		t.Fatal(err)
	}
	age := 12
	s.store.UpdateUser(ctx, f.kid.ID, store.UserUpdate{MaxAge: &age})
	f.admin, err = s.store.CreateUser(ctx, "Admin", true, "rose", "", "")
	if err != nil {
		t.Fatal(err)
	}
	f.lib, err = s.store.SyncLibrary(ctx, "Fixture films", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	add := func(title, rating string) string {
		t.Helper()
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: f.lib, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		file, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: f.lib, Path: "/fixture/" + title + ".mp4"}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.store.SetFileItems(ctx, file, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		if err = s.store.EditItem(ctx, id, store.ItemEdit{ContentRating: &rating}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	f.movie = add("Family fixture", "PG")
	f.adult = add("Hidden adult title", "18")
	f.labelled = add("Hidden label title", "PG")
	for token, user := range map[string]*store.User{"host": u, "host-second": u, "friend": f.friend, "child": f.kid, "admin": f.admin} {
		via := "password"
		if user == f.friend {
			via = "friend"
		}
		if err = s.store.CreateSession(ctx, auth.HashToken(token), user.ID, via, ""); err != nil {
			t.Fatal(err)
		}
	}
	f.handler = New(Deps{Store: s.store, Settings: s.settings, Log: s.log, WatchTogether: f.manager})
	s.watchTogether = f.manager
	return f
}
func (f *watchFixture) request(method, path, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://arc/api/v1"+path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:12345"
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}
func (f *watchFixture) create(t *testing.T, item string) watchInviteDTO {
	t.Helper()
	w := f.request("POST", "/watch-together", "host", `{"item_id":"`+item+`"}`)
	var out watchInviteDTO
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != 201 {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	return out
}
func decodeWatchRoom(t *testing.T, w *httptest.ResponseRecorder) watchRoomDTO {
	t.Helper()
	var out watchRoomDTO
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != 200 {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	return out
}
func TestWatchTogetherPermissionsPlaceholdersAndConflicts(t *testing.T) {
	f := newWatchFixture(t)
	invite := f.create(t, f.movie)
	id := invite.Room.ID
	if own := decodeWatchRoom(t, f.request("GET", "/watch-together/current", "host", "")); own.ID != id {
		t.Fatal("own room recovery failed")
	}
	if invite.Room.Started || !strings.Contains(f.request("GET", "/watch-together/"+id, "host", "").Body.String(), `"started":false`) {
		t.Fatal("a new room says it has started")
	}
	if w := f.request("GET", "/watch-together/current", "admin", ""); w.Code != 404 {
		t.Fatal("current exposed other room")
	}
	path := "/watch-together/" + id
	for _, token := range []string{"friend", "child", "host-second"} {
		room := decodeWatchRoom(t, f.request("POST", "/watch-together/join", token, `{"code":"`+invite.InviteCode+`"}`))
		if !room.Available {
			t.Fatal("allowed join denied")
		}
	}
	room := decodeWatchRoom(t, f.request("GET", path, "host", ""))
	if len(room.Participants) != 4 {
		t.Fatal("same-profile devices collapsed")
	}
	// Admin role alone cannot inspect or join a room by ID. No list endpoint exists.
	if w := f.request("GET", path, "admin", ""); w.Code != 404 {
		t.Fatal("admin saw private room", w.Code)
	}
	if w := f.request("GET", "/watch-together", "admin", ""); w.Code != 405 && w.Code != 404 {
		t.Fatal("room list exposed")
	}
	if w := f.request("GET", path, "", ""); w.Code != 401 {
		t.Fatal("unauth room")
	}
	w := f.request("POST", path+"/commands", "host", fmt.Sprintf(`{"revision":%d,"action":"change_item","item_id":"%s"}`, room.Revision, f.adult))
	room = decodeWatchRoom(t, w)
	kid := decodeWatchRoom(t, f.request("GET", path, "child", ""))
	if kid.Available || kid.ItemID != "" || kid.Item != nil || strings.Contains(f.request("GET", path, "child", "").Body.String(), f.adult) {
		t.Fatal("restricted metadata leaked", kid)
	}
	if len(kid.Participants) != 4 {
		t.Fatal("restricted participant silently dropped")
	}
	w = f.request("POST", path+"/commands", "child", fmt.Sprintf(`{"revision":%d,"action":"play"}`, kid.Revision))
	if w.Code != 404 {
		t.Fatal("restricted participant controls hidden title")
	}
	w = f.request("POST", path+"/commands", "host", `{"revision":1,"action":"pause"}`)
	var conflict watchConflictDTO
	json.Unmarshal(w.Body.Bytes(), &conflict)
	if w.Code != 409 || conflict.Room.ID != id || conflict.Room.Revision < 2 {
		t.Fatal("stale response", w.Code, w.Body.String())
	}
	w = f.request("POST", path+"/commands", "child", `{"revision":1,"action":"change_item","item_id":"`+f.movie+`"}`)
	conflict = watchConflictDTO{}
	json.Unmarshal(w.Body.Bytes(), &conflict)
	if w.Code != 409 || conflict.Room.ItemID != "" || conflict.Room.Item != nil {
		t.Fatal("409 leaked hidden title", w.Body.String())
	}
	// Independently enforce library and label restrictions when joining.
	none := []int64{}
	f.s.store.UpdateUser(t.Context(), f.friend.ID, store.UserUpdate{LibraryIDs: &none})
	if w = f.request("POST", "/watch-together/join", "friend", `{"code":"`+invite.InviteCode+`"}`); w.Code != 404 {
		t.Fatal("library restriction bypass")
	}
	if w = f.request("POST", "/watch-together/join", "child", `{"code":"`+invite.InviteCode+`"}`); w.Code != 404 {
		t.Fatal("age restriction bypass")
	}
	f.s.store.UpdateUser(t.Context(), f.friend.ID, store.UserUpdate{LibraryIDs: &[]int64{f.lib}})
	// Labels are applied through the existing item edit API/store helper.
	labels := []string{"private"}
	if err := f.s.store.EditItem(t.Context(), f.labelled, store.ItemEdit{Labels: &labels}); err != nil {
		t.Fatal(err)
	}
	f.s.store.UpdateUser(t.Context(), f.friend.ID, store.UserUpdate{ExcludeLabels: &labels})
	room = decodeWatchRoom(t, f.request("GET", path, "host", ""))
	room = decodeWatchRoom(t, f.request("POST", path+"/commands", "host", fmt.Sprintf(`{"revision":%d,"action":"change_item","item_id":"%s"}`, room.Revision, f.labelled)))
	friend := decodeWatchRoom(t, f.request("GET", path, "friend", ""))
	if friend.Available || friend.Item != nil {
		t.Fatal("label restriction bypass")
	}
	// A profile switch cannot inherit the old profile's room, even with the same device session.
	f.s.store.SetSessionUser(t.Context(), auth.HashToken("host-second"), f.admin.ID)
	if w = f.request("GET", path, "host-second", ""); w.Code != 404 {
		t.Fatal("profile switch inherited membership")
	}
}
func TestWatchTogetherBodyBoundsCodeGuessingAndTime(t *testing.T) {
	f := newWatchFixture(t)
	for _, body := range []string{`{}`, `{"item_id":"x","start_ms":-1}`, `{"item_id":"x","unknown":true}`, `{"item_id":"x"} {}`, strings.Repeat("x", 5000)} {
		w := f.request("POST", "/watch-together", "friend", body)
		if w.Code != 400 && w.Code != 404 {
			t.Fatal("invalid body accepted", w.Code)
		}
	}
	w := f.request("GET", "/watch-together/time?client_sent_ms=123", "friend", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"client_sent_ms":123`) {
		t.Fatal(w.Body.String())
	}
	for range 5 {
		w = f.request("POST", "/watch-together/join", "child", `{"code":"WRONG123"}`)
		if w.Code != 404 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w = f.request("POST", "/watch-together/join", "child", `{"code":"WRONG123"}`); w.Code != 429 {
		t.Fatal("guessing not rate limited")
	}
	if w = f.request("GET", "/watch-together/time?client_sent_ms=no", "friend", ""); w.Code != 400 {
		t.Fatal("bad clock input accepted")
	}
}
func TestWatchTogetherSSEDeliveryBoundsAndRevocation(t *testing.T) {
	f := newWatchFixture(t)
	invite := f.create(t, f.movie)
	id := invite.Room.ID
	server := httptest.NewServer(f.handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/v1/watch-together/"+id+"/events", nil)
	r.Header.Set("Authorization", "Bearer host")
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal(response.Status)
	}
	reader := bufio.NewReader(response.Body)
	read := func() watchRoomDTO {
		t.Helper()
		var data string
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(line, "data: ") {
				data = strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			}
			if line == "\n" && data != "" {
				var d watchRoomDTO
				if err = json.Unmarshal([]byte(data), &d); err != nil {
					t.Fatal(err)
				}
				return d
			}
		}
	}
	initial := read()
	if initial.ID != id || initial.ServerTimeMS <= 0 {
		t.Fatal(initial)
	}
	// One stream per participant: a reconnect replaces the older stream (which may be a half-open
	// connection the server can't detect yet) rather than being refused.
	r2, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/v1/watch-together/"+id+"/events", nil)
	r2.Header.Set("Authorization", "Bearer host")
	second, err := client.Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Body.Close()
	if second.StatusCode != 200 {
		t.Fatal("a reconnect was refused", second.StatusCode)
	}
	replacedAt := time.Now()
	for {
		if _, err = reader.ReadString('\n'); err != nil {
			break
		}
	}
	if time.Since(replacedAt) > 2*time.Second {
		t.Fatal("the replaced stream didn't end promptly")
	}
	reader = bufio.NewReader(second.Body)
	if again := read(); again.ID != id {
		t.Fatal(again)
	}
	start := time.Now()
	changed, err := f.manager.Command(id, (watchtogether.Actor{Session: auth.HashToken("host"), UserID: f.host.ID}).Key(), initial.Revision, "play", "", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	for {
		d := read()
		if d.Revision >= changed.Revision {
			if time.Since(start) > time.Second {
				t.Fatal("room change delivery stalled")
			}
			break
		}
	}
	revokedAt := time.Now()
	if err = f.s.store.DeleteSession(t.Context(), auth.HashToken("host")); err != nil {
		t.Fatal(err)
	}
	for {
		_, err = reader.ReadString('\n')
		if err != nil {
			break
		}
	}
	if time.Since(revokedAt) > 3*time.Second {
		t.Fatal("revoked stream stayed open", err)
	}
}

func TestWatchTogetherHeartbeatsHostLeaveDashboardAndAccessChanges(t *testing.T) {
	f := newWatchFixture(t)
	invite := f.create(t, f.movie)
	id := invite.Room.ID
	path := "/watch-together/" + id
	room := decodeWatchRoom(t, f.request("POST", "/watch-together/join", "friend", `{"code":"`+invite.InviteCode+`"}`))
	room = decodeWatchRoom(t, f.request("POST", path+"/heartbeat", "host", fmt.Sprintf(`{"generation":%d,"position_ms":70000,"state":"paused","ready":true}`, room.Generation)))
	if room.PositionMS != 0 {
		t.Fatal("heartbeat changed authoritative position")
	}
	// Room readiness never writes ordinary personal watch progress or preferences.
	data, _ := f.s.store.UserDataFor(t.Context(), f.host.ID, []string{f.movie})
	if data[f.movie].PositionMS != 0 || data[f.movie].Played {
		t.Fatal("room wrote watch history")
	}
	room = decodeWatchRoom(t, f.request("POST", path+"/commands", "friend", fmt.Sprintf(`{"revision":%d,"action":"play"}`, room.Revision)))
	if room.State != "waiting" {
		t.Fatal("unready friend did not wait")
	}
	room = decodeWatchRoom(t, f.request("POST", path+"/heartbeat", "friend", fmt.Sprintf(`{"generation":%d,"position_ms":0,"state":"playing","ready":true}`, room.Generation)))
	if room.State != "playing" {
		t.Fatal("ready room did not resume")
	}
	// Correlation/count are additive to existing Playing only, with no new admin room contents API.
	r := httptest.NewRequest("PUT", "/", nil)
	r = r.WithContext(context.WithValue(r.Context(), userKey, f.host))
	r = r.WithContext(context.WithValue(r.Context(), sessionKey, auth.HashToken("host")))
	f.s.playing.setMethod(r, f.movie, "direct", "", streamDetail{}, nil)
	w := httptest.NewRecorder()
	f.s.listPlaying(w, r)
	var plays []playbackDTO
	json.Unmarshal(w.Body.Bytes(), &plays)
	if len(plays) != 1 || plays[0].WatchTogetherRoomID != id || plays[0].WatchTogetherParticipants != 2 {
		t.Fatal("dashboard correlation", w.Body.String())
	}
	// Permission changes during the same title are enforced on GET and remove readiness blockers.
	none := []int64{}
	if err := f.s.store.UpdateUser(t.Context(), f.friend.ID, store.UserUpdate{LibraryIDs: &none}); err != nil {
		t.Fatal(err)
	}
	friend := decodeWatchRoom(t, f.request("GET", path, "friend", ""))
	if friend.Available || friend.Item != nil {
		t.Fatal("live library permission change ignored")
	}
	if w = f.request("POST", path+"/leave", "host", ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	room = decodeWatchRoom(t, f.request("GET", path, "friend", ""))
	if room.HostID != room.SelfID || len(room.Participants) != 1 {
		t.Fatal("host transfer", room)
	}
	// Host can end the room even after losing title access.
	room = decodeWatchRoom(t, f.request("POST", path+"/commands", "friend", fmt.Sprintf(`{"revision":%d,"action":"end"}`, room.Revision)))
	if room.State != "ended" || room.Item != nil {
		t.Fatal(room)
	}
}

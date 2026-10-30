package api

import (
	"ams/internal/auth"
	"ams/internal/store"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlayQueueAPIIsolationSnapshotAndAccessChanges(t *testing.T) {
	s, user := securityServer(t)
	ctx := t.Context()
	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := 0; i < 3; i++ {
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: fmt.Sprintf("Private title %d", i)}, fmt.Sprint(i))
		if err != nil {
			t.Fatal(err)
		}
		f, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: filepath.Join(t.TempDir(), fmt.Sprintf("generated%d.mp4", i)), Size: 1}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.store.SetFileItems(ctx, f, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	other, err := s.store.CreateUser(ctx, "Other", false, "teal", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for token, profile := range map[string]string{"device-a": user.ID, "device-b": user.ID, "other-profile": other.ID} {
		if err = s.store.CreateSession(ctx, auth.HashToken(token), profile, "password", "queue test"); err != nil {
			t.Fatal(err)
		}
	}
	playlist, err := s.store.CreatePlaylist(ctx, user.ID, "Snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.AddToPlaylist(ctx, playlist.ID, []string{ids[0], ids[1], ids[0]}); err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://arc/api/v1"+path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/play-queues/current", "", ""); w.Code != 401 {
		t.Fatalf("anonymous %d", w.Code)
	}
	w := request("POST", "/play-queues", `{"playlist_id":"`+playlist.ID+`"}`, "device-a")
	if w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	var q playQueueDTO
	if err = json.Unmarshal(w.Body.Bytes(), &q); err != nil {
		t.Fatal(err)
	}
	if len(q.Entries) != 3 || q.Entries[0].ID == q.Entries[2].ID || q.Entries[0].ItemID != q.Entries[2].ItemID {
		t.Fatal("duplicate snapshot entries wrong")
	}
	if err = s.store.AddToPlaylist(ctx, playlist.ID, []string{ids[2]}); err != nil {
		t.Fatal(err)
	}
	w = request("GET", "/play-queues/current", "", "device-a")
	var recovered playQueueDTO
	if err = json.Unmarshal(w.Body.Bytes(), &recovered); err != nil {
		t.Fatal(err)
	}
	if recovered.ID != q.ID || len(recovered.Entries) != 3 {
		t.Fatal("playlist edit changed live snapshot")
	}
	for _, token := range []string{"device-b", "other-profile"} {
		for _, method := range []string{"GET", "PATCH", "DELETE"} {
			path := "/play-queues/" + q.ID
			if method == "DELETE" {
				path += "?revision=1"
			}
			w = request(method, path, `{"operation":"clear","revision":1}`, token)
			if w.Code != 404 {
				t.Fatalf("cross-device/profile %s %d %s", method, w.Code, w.Body.String())
			}
		}
	}
	w = request("PATCH", "/play-queues/"+q.ID, `{"revision":1,"operation":"next","item_ids":["`+ids[2]+`"]}`, "device-a")
	if w.Code != 200 {
		t.Fatalf("play next %d %s", w.Code, w.Body.String())
	}
	w = request("PATCH", "/play-queues/"+q.ID, `{"revision":1,"operation":"clear"}`, "device-a")
	if w.Code != 409 {
		t.Fatalf("stale edit %d", w.Code)
	}
	// Restrictions are checked on reads, not just at creation. Placeholders must not leak titles,
	// item IDs, source paths or provider metadata retained in the stored queue.
	none := []int64{}
	if err = s.store.UpdateUser(ctx, user.ID, store.UserUpdate{LibraryIDs: &none}); err != nil {
		t.Fatal(err)
	}
	w = request("GET", "/play-queues/current", "", "device-a")
	if w.Code != 200 {
		t.Fatalf("restricted own queue %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "Private title") || strings.Contains(w.Body.String(), ids[0]) || strings.Contains(w.Body.String(), "generated") {
		t.Fatal("restricted queue leaked metadata")
	}
	var hidden playQueueDTO
	if err = json.Unmarshal(w.Body.Bytes(), &hidden); err != nil {
		t.Fatal(err)
	}
	for _, e := range hidden.Entries {
		if e.Available || e.Item != nil || e.ItemID != "" {
			t.Fatal("hidden item payload retained")
		}
	}
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("private queue cache policy missing")
	}
	w = request("POST", "/play-queues", `{"item_ids":["`+ids[0]+`"]}`, "device-a")
	if w.Code != 404 {
		t.Fatalf("created queue with restricted item %d", w.Code)
	}
	// Revocation cascades queue storage and prevents subsequent API use.
	if err = s.store.DeleteSession(ctx, auth.HashToken("device-a")); err != nil {
		t.Fatal(err)
	}
	if w = request("GET", "/play-queues/current", "", "device-a"); w.Code != 401 {
		t.Fatalf("revocation %d", w.Code)
	}
}

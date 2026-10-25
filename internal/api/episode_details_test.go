package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"ams/internal/auth"
	"ams/internal/store"
)

func TestInlineSpecialsAPITraversalAndAccess(t *testing.T) {
	ctx := context.Background()
	s, viewer := securityServer(t)
	admin, err := s.store.CreateUser(ctx, "Owner", true, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	limited, err := s.store.CreateUser(ctx, "Limited", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	otherLibs := []int64{999}
	if err := s.store.UpdateUser(ctx, limited.ID, store.UserUpdate{LibraryIDs: &otherLibs}); err != nil {
		t.Fatal(err)
	}
	for token, id := range map[string]string{"viewer": viewer.ID, "admin": admin.ID, "limited": limited.ID} {
		if err := s.store.CreateSession(ctx, auth.HashToken(token), id, "password", "fixture"); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := s.store.SyncLibrary(ctx, "Shows", "shows", "tvdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	series, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: "Show"}, "series")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for i, title := range []string{"One", "Two", "Interlude", "Unknown"} {
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, ParentID: series, Kind: "episode", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		f := &store.MediaFile{LibraryID: lib, Path: t.TempDir() + "/" + title + ".mp4", Size: 1, DurationMS: 120000}
		f.ID, err = s.store.SaveFile(ctx, f, int64(i+1), 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetFileItems(ctx, f.ID, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
	}
	oid, err := s.store.UpsertOrdering(ctx, series, "tvdb_aired", "", "Aired", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.ReplaceOrdering(ctx, oid, map[int]string{0: "Specials", 1: "Season 1"}, nil, []store.OrderingEntry{
		{EpisodeID: ids[0], Season: 1, Episode: 1}, {EpisodeID: ids[1], Season: 1, Episode: 2}, {EpisodeID: ids[2], Season: 0, Episode: 1}, {EpisodeID: ids[3], Season: 0, Episode: 2},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.store.ReplaceSpecialPlacements(ctx, series, map[string]string{ids[2]: ids[1]}); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetItemDetails(ctx, ids[2], nil, []store.Credit{{TMDBID: 71, Name: "Guest", Role: "actor"}, {TMDBID: 72, Name: "Director", Role: "director"}}); err != nil {
		t.Fatal(err)
	}
	call := func(token, method, id, body string, handler http.HandlerFunc) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "/api/test", strings.NewReader(body))
		r.SetPathValue("id", id)
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		w := httptest.NewRecorder()
		s.signedIn(handler)(w, r)
		return w
	}
	verify := func(w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatalf("HTTP %d want %d: %s", w.Code, code, w.Body.String())
		}
	}
	w := call("viewer", "GET", series, "", s.listOrderings)
	verify(w, 200)
	if !strings.Contains(w.Body.String(), `"inline_specials":false`) {
		t.Fatal(w.Body.String())
	}
	w = call("admin", "PATCH", series, `{"inline_specials":true}`, s.setSpecials)
	verify(w, 200)
	for _, bad := range []string{`{}`, `{"inline_specials":"yes"}`, `{"inline_specials":1}`} {
		verify(call("viewer", "PATCH", series, bad, s.setSpecials), 400)
	}
	w = call("viewer", "GET", series, "", s.seasons)
	verify(w, 200)
	var response struct {
		Seasons []seasonDTO `json:"seasons"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Seasons) != 2 || len(response.Seasons[0].Episodes) != 3 || response.Seasons[0].Episodes[1].ID != ids[2] || response.Seasons[0].Episodes[1].SeasonNumber != 0 || !response.Seasons[0].Episodes[1].InlineSpecial {
		t.Fatalf("seasons=%s", w.Body.String())
	}
	for current, want := range map[string]string{ids[0]: ids[2], ids[2]: ids[1]} {
		w = call("viewer", "GET", current, "", s.nextEpisode)
		verify(w, 200)
		var d itemDTO
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil || d.ID != want {
			t.Fatalf("next=%s %v", w.Body.String(), err)
		}
	}
	w = call("viewer", "GET", ids[2], "", s.getItem)
	verify(w, 200)
	if !strings.Contains(w.Body.String(), `"name":"Guest"`) || strings.Contains(w.Body.String(), `"episode_count":0`) {
		t.Fatal(w.Body.String())
	}
	// The same traversal drives expansion of a series into a queue and season-wide actions.
	var queue []string
	w = call("viewer", "GET", series, "", func(w http.ResponseWriter, r *http.Request) {
		it, _ := s.store.GetItem(ctx, series)
		var err error
		queue, err = s.orderedEpisodeIDs(r, it)
		if err != nil {
			t.Fatal(err)
		}
	})
	verify(w, 200)
	if !slices.Equal(queue, []string{ids[0], ids[2], ids[1], ids[3]}) {
		t.Fatalf("queue=%v", queue)
	}
	w = call("viewer", "GET", series, "", func(w http.ResponseWriter, r *http.Request) {
		it, _ := s.store.GetItem(ctx, series)
		season := 1
		got, err := s.seriesEpisodeIDs(r, it, &season)
		if err != nil || !slices.Equal(got, []string{ids[0], ids[2], ids[1]}) {
			t.Fatalf("season scope=%v %v", got, err)
		}
	})
	verify(w, 200)
	if _, err := s.store.ReportProgress(ctx, viewer.ID, ids[0], 119999, 120000); err != nil {
		t.Fatal(err)
	}
	w = call("viewer", "GET", series, "", s.nextUp)
	verify(w, 200)
	var next []itemDTO
	if err := json.Unmarshal(w.Body.Bytes(), &next); err != nil || len(next) != 1 || next[0].ID != ids[2] {
		t.Fatalf("Next Up=%s %v", w.Body.String(), err)
	}
	w = call("viewer", "PATCH", series, `{"inline_specials":false}`, s.setSpecials)
	verify(w, 200)
	w = call("viewer", "GET", ids[0], "", s.nextEpisode)
	verify(w, 200)
	var d itemDTO
	json.Unmarshal(w.Body.Bytes(), &d)
	if d.ID != ids[1] {
		t.Fatal("profile override ignored")
	}
	verify(call("viewer", "PATCH", series, `{"inline_specials":null}`, s.setSpecials), 200)
	verify(call("limited", "GET", ids[2], "", s.getItem), 404)
	verify(call("limited", "PATCH", series, `{"inline_specials":true}`, s.setSpecials), 404)
}

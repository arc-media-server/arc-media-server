package api

import (
	"ams/internal/auth"
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestItemsPagingRejectsInvalidBounds(t *testing.T) {
	s, u := securityServer(t)
	if err := s.store.CreateSession(t.Context(), auth.HashToken("paging-fixture"), u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	for _, prefix := range []string{"/api/items", "/api/v1/items"} {
		for _, query := range []string{"limit=600", "limit=wat", "limit=-1", "limit=0", "limit=99999999999999999999", "offset=-1", "offset=10000001", "offset=wat"} {
			r := httptest.NewRequest("GET", prefix+"?"+query, nil)
			r.Header.Set("Authorization", "Bearer paging-fixture")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 400 {
				t.Errorf("%s?%s: %d; wanted 400", prefix, query, w.Code)
			}
		}
		for _, query := range []string{"", "limit=500", "limit=1&offset=0"} {
			r := httptest.NewRequest("GET", prefix+"?"+query, nil)
			r.Header.Set("Authorization", "Bearer paging-fixture")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 200 {
				t.Errorf("valid %s?%s: %d %s", prefix, query, w.Code, w.Body.String())
			}
		}
	}
}

func TestPlaylistPatchValidatesEntireRequestBeforeRename(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		smart      bool
		status     int
	}{
		{"invalid smart filter", `{"title":"Changed","filter":{"library_id":0}}`, true, 400},
		{"rules on ordinary playlist", `{"title":"Changed","filter":{"library_id":0}}`, false, 409},
		{"title too long", fmt.Sprintf(`{"title":%q}`, strings.Repeat("é", 101)), false, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, u := securityServer(t)
			p, err := s.store.CreatePlaylist(t.Context(), u.ID, "Original")
			if err != nil {
				t.Fatal(err)
			}
			originalFilter := ""
			if tc.smart {
				originalFilter = `{"library_id":123}`
				if err = s.store.SetPlaylistFilter(t.Context(), p.ID, originalFilter); err != nil {
					t.Fatal(err)
				}
			}
			r := httptest.NewRequest("PATCH", "/api/playlists/"+p.ID, strings.NewReader(tc.body))
			r.SetPathValue("id", p.ID)
			r = r.WithContext(context.WithValue(r.Context(), userKey, u))
			w := httptest.NewRecorder()
			s.updatePlaylist(w, r)
			if w.Code != tc.status {
				t.Errorf("status %d want %d: %s", w.Code, tc.status, w.Body.String())
			}
			saved, err := s.store.GetPlaylist(t.Context(), p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if saved.Title != "Original" || saved.Filter != originalFilter {
				t.Errorf("rejected patch changed playlist: %+v", saved)
			}
		})
	}
}

func TestPlaylistPatchAcceptsHundredUnicodeCharactersAndNewFilter(t *testing.T) {
	s, u := securityServer(t)
	lib, err := s.store.SyncLibrary(t.Context(), "Fixture", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.store.CreatePlaylist(t.Context(), u.ID, "Original")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetPlaylistFilter(t.Context(), p.ID, `{"library_id":123}`); err != nil {
		t.Fatal(err)
	}
	title := strings.Repeat("é", 100)
	r := httptest.NewRequest("PATCH", "/", strings.NewReader(fmt.Sprintf(`{"title":%q,"filter":{"library_id":%d}}`, " "+title+" ", lib)))
	r.SetPathValue("id", p.ID)
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	w := httptest.NewRecorder()
	s.updatePlaylist(w, r)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved, err := s.store.GetPlaylist(t.Context(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Title != title || !strings.Contains(saved.Filter, fmt.Sprintf(`"library_id":%d`, lib)) {
		t.Fatalf("valid combined update missing: %+v", saved)
	}
}

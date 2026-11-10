package api

import (
	"ams/internal/auth"
	"ams/internal/store"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlaylistSeriesExpansionDoesNotExposeHiddenCountsOrStoreItems(t *testing.T) {
	for _, restriction := range []string{"library", "label", "age"} {
		t.Run(restriction, func(t *testing.T) {
			s, u := securityServer(t)
			ctx := t.Context()
			public, err := s.store.SyncLibrary(ctx, "Public", "shows", "tvdb_aired", nil)
			if err != nil {
				t.Fatal(err)
			}
			hidden, err := s.store.SyncLibrary(ctx, "Private", "shows", "tvdb_aired", nil)
			if err != nil {
				t.Fatal(err)
			}
			if restriction != "library" {
				hidden = public
			}
			makeShow := func(lib int64, title string, count int) string {
				t.Helper()
				show, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: title}, title)
				if err != nil {
					t.Fatal(err)
				}
				for i := 0; i < count; i++ {
					episode, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, ParentID: show, Kind: "episode", Title: "Episode"}, fmt.Sprintf("%s:%d", title, i))
					if err != nil {
						t.Fatal(err)
					}
					file, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: fmt.Sprintf("/fixture/%s/%d.mkv", title, i), Size: 1}, 1, 1)
					if err != nil {
						t.Fatal(err)
					}
					if err = s.store.SetFileItems(ctx, file, []string{episode}, 0); err != nil {
						t.Fatal(err)
					}
				}
				return show
			}
			visibleShow := makeShow(public, "Visible", 1)
			hiddenShow := makeShow(hidden, "Hidden", 3)
			if restriction == "library" {
				ids := []int64{public}
				if err = s.store.UpdateUser(ctx, u.ID, store.UserUpdate{LibraryIDs: &ids}); err != nil {
					t.Fatal(err)
				}
			} else if restriction == "label" {
				labels := []string{"blocked"}
				if err = s.store.EditItem(ctx, hiddenShow, store.ItemEdit{Labels: &labels}); err != nil {
					t.Fatal(err)
				}
				if err = s.store.UpdateUser(ctx, u.ID, store.UserUpdate{ExcludeLabels: &labels}); err != nil {
					t.Fatal(err)
				}
			}
			if restriction == "age" {
				visibleRating, hiddenRating := "U", "18"
				if err = s.store.EditItem(ctx, visibleShow, store.ItemEdit{ContentRating: &visibleRating}); err != nil {
					t.Fatal(err)
				}
				if err = s.store.EditItem(ctx, hiddenShow, store.ItemEdit{ContentRating: &hiddenRating}); err != nil {
					t.Fatal(err)
				}
				age := 12
				if err = s.store.UpdateUser(ctx, u.ID, store.UserUpdate{MaxAge: &age}); err != nil {
					t.Fatal(err)
				}
			}
			p, err := s.store.CreatePlaylist(ctx, u.ID, "Fixture")
			if err != nil {
				t.Fatal(err)
			}
			if err = s.store.CreateSession(ctx, auth.HashToken("fixture"), u.ID, "password", ""); err != nil {
				t.Fatal(err)
			}
			h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
			request := func(ids []string) *httptest.ResponseRecorder {
				payload, err := json.Marshal(map[string]any{"item_ids": ids})
				if err != nil {
					t.Fatal(err)
				}
				r := httptest.NewRequest("POST", "http://arc/api/v1/playlists/"+p.ID+"/items", strings.NewReader(string(payload)))
				r.Header.Set("Authorization", "Bearer fixture")
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				return w
			}
			hiddenResponse := request([]string{visibleShow, hiddenShow})
			missingResponse := request([]string{"missing"})
			if hiddenResponse.Code != 400 || hiddenResponse.Code != missingResponse.Code {
				t.Errorf("hidden series revealed count: %d %s", hiddenResponse.Code, hiddenResponse.Body.String())
			}
			var hiddenError, missingError map[string]any
			json.Unmarshal(hiddenResponse.Body.Bytes(), &hiddenError)
			json.Unmarshal(missingResponse.Body.Bytes(), &missingError)
			if hiddenError["error"] != missingError["error"] {
				t.Errorf("hidden response differs from missing: %v vs %v", hiddenError, missingError)
			}
			_, items, err := s.store.PlaylistEntries(ctx, p.ID, store.Access{})
			if err != nil || len(items) != 0 {
				t.Errorf("failed mixed selection wrote entries: %d (%v)", len(items), err)
			}
			visibleResponse := request([]string{visibleShow})
			if visibleResponse.Code != 200 {
				t.Fatal("visible show refused", visibleResponse.Code, visibleResponse.Body.String())
			}
			var result struct {
				Added int `json:"added"`
			}
			if err = json.Unmarshal(visibleResponse.Body.Bytes(), &result); err != nil || result.Added != 1 {
				t.Fatal("visible expansion changed", result, err)
			}
		})
	}
}

package api

import (
	"ams/internal/auth"
	"ams/internal/store"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlaybackPreviewAlwaysReturnsEmptyListsAsArrays(t *testing.T) {
	for _, state := range []string{"needs_analysis", "no_video", "direct"} {
		t.Run(state, func(t *testing.T) {
			s, u := securityServer(t)
			lib, err := s.store.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
			if err != nil {
				t.Fatal(err)
			}
			item, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			f := store.MediaFile{LibraryID: lib, Path: "/nonexistent/fixture.mp4", Container: "mp4", DurationMS: 60000, Streams: []store.Stream{{Kind: "audio", Index: 0, Codec: "aac", Channels: 2}}}
			var probed int64 = 1
			if state == "needs_analysis" {
				probed = 0
			}
			if state == "direct" {
				f.Streams = append(f.Streams, store.Stream{Kind: "video", Index: 1, Codec: "h264", Width: 640, Height: 360, BitDepth: 8})
			}
			file, err := s.store.SaveFile(t.Context(), &f, probed, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.store.SetFileItems(t.Context(), file, []string{item}, 0); err != nil {
				t.Fatal(err)
			}
			if err = s.store.CreateSession(t.Context(), auth.HashToken("fixture"), u.ID, "password", ""); err != nil {
				t.Fatal(err)
			}
			h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
			r := httptest.NewRequest("POST", "http://arc/api/v1/items/"+item+"/playback-preview", strings.NewReader(`{"caps":{"containers":["mp4"],"video":["h264"],"audio":["aac"]}}`))
			r.Header.Set("Authorization", "Bearer fixture")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			}
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(w.Body.Bytes(), &fields); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"reasons", "reason_codes", "audio_tracks", "subtitle_tracks", "versions", "chapters", "markers"} {
				if field == "reason_codes" && len(fields[field]) == 0 {
					continue
				} // optional via omitempty
				if len(fields[field]) == 0 || fields[field][0] != '[' {
					t.Errorf("%s must be an array, got %s", field, fields[field])
				}
			}
		})
	}
}

func TestProfileNullAndEmptyLibrariesHaveDifferentMeanings(t *testing.T) {
	for _, ids := range [][]int64{nil, {}, {1, 2}} {
		data, err := json.Marshal(toProfile(store.User{LibraryIDs: ids}))
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		if ids == nil && string(fields["library_ids"]) != "null" {
			t.Fatal("all-libraries semantics changed", string(data))
		}
		if ids != nil && fields["library_ids"][0] != '[' {
			t.Fatal("explicit library selection lost", string(data))
		}
	}
}

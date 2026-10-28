package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ams/internal/store"
)

// An Other videos library is browsed folder by folder: folders with their total counts and a
// cover, the videos directly in each folder oldest first, and the trail back up.
func TestLibraryFolders(t *testing.T) {
	s, u := securityServer(t)
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "Home Videos")
	lib, err := s.store.SyncLibrary(ctx, "Home Videos", "videos", "tmdb_aired", []string{root})
	if err != nil {
		t.Fatal(err)
	}
	add := func(rel, title, date string) string {
		t.Helper()
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: title}, "video:"+rel)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetLocalVideo(ctx, id, title, date); err != nil {
			t.Fatal(err)
		}
		fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: filepath.Join(root, filepath.FromSlash(rel)), Size: 1, ModTime: 1}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		return id
	}
	loose := add("loose.mp4", "Loose", "2018-01-01")
	beach := add("Holidays/2019/beach.mp4", "Beach", "2019-07-14")
	add("Holidays/2019/pier.mp4", "Pier", "2019-07-15")
	add("Holidays/2020/snow.mp4", "Snow", "2020-01-02")
	first := add("Holidays/arrival.mp4", "Arrival", "2019-07-01")

	type folders struct {
		Path    string                        `json:"path"`
		Trail   []struct{ Name, Path string } `json:"trail"`
		Folders []folderDTO                   `json:"folders"`
		Items   []struct{ ID, Title string }  `json:"items"`
	}
	get := func(path string) (int, folders) {
		r := httptest.NewRequest("GET", "/api/libraries/x/folders?path="+path, nil)
		r.SetPathValue("id", jsonInt(lib))
		r = r.WithContext(context.WithValue(r.Context(), userKey, u))
		w := httptest.NewRecorder()
		s.libraryFolders(w, r)
		var out folders
		json.NewDecoder(w.Body).Decode(&out)
		return w.Code, out
	}
	// One library folder: the top is inside it.
	code, top := get("")
	if code != http.StatusOK || len(top.Items) != 1 || top.Items[0].ID != loose || len(top.Folders) != 1 {
		t.Fatalf("top = %d %+v", code, top)
	}
	holidays := top.Folders[0]
	if holidays.Name != "Holidays" || holidays.Count != 4 || holidays.CoverID != first || holidays.Path != "0/Holidays" {
		t.Errorf("Holidays = %+v (oldest video should be its cover)", holidays)
	}
	_, h := get(holidays.Path)
	if len(h.Items) != 1 || h.Items[0].Title != "Arrival" || len(h.Folders) != 2 || h.Folders[0].Name != "2019" || h.Folders[0].Count != 2 {
		t.Errorf("Holidays contents = %+v", h)
	}
	if len(h.Trail) != 1 || h.Trail[0].Name != "Holidays" {
		t.Errorf("trail = %+v", h.Trail)
	}
	_, y := get("0/Holidays/2019")
	if len(y.Items) != 2 || y.Items[0].ID != beach || y.Items[1].Title != "Pier" {
		t.Errorf("2019 = %+v, want oldest first", y.Items)
	}
	if code, _ := get("0/Nowhere"); code != http.StatusNotFound {
		t.Errorf("unknown folder = %d", code)
	}
	// Only Other videos libraries have folders.
	movies, _ := s.store.SyncLibrary(ctx, "Films", "movies", "tmdb_aired", []string{t.TempDir()})
	r := httptest.NewRequest("GET", "/api/libraries/x/folders", nil)
	r.SetPathValue("id", jsonInt(movies))
	w := httptest.NewRecorder()
	s.libraryFolders(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	if w.Code != http.StatusBadRequest {
		t.Errorf("movie library folders = %d", w.Code)
	}
}

func jsonInt(n int64) string { b, _ := json.Marshal(n); return string(b) }

package api

import (
	"ams/internal/auth"
	"ams/internal/downloads"
	"ams/internal/store"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadPreparationRequiresExplicitPermission(t *testing.T) {
	s, u := securityServer(t)
	if err := s.store.CreateSession(t.Context(), auth.HashToken("download-viewer"), u.ID, "password", "fixture"); err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	r := httptest.NewRequest("POST", "http://arc/api/v1/downloads", strings.NewReader(`{"item_id":"private","file_id":1,"mode":"original","idempotency_key":"one"}`))
	r.Header.Set("Authorization", "Bearer download-viewer")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("ordinary profile without grant: got %d, want403", w.Code)
	}
}

func TestDownloadPreparationAPIContractsAndIsolation(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	yes := true
	if err := s.store.UpdateUser(ctx, u.ID, store.UserUpdate{AllowDownloads: &yes}); err != nil {
		t.Fatal(err)
	}
	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Generated title"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "private-file.mp4")
	if err = os.WriteFile(path, []byte("generated source"), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	f := store.MediaFile{LibraryID: lib, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix()}
	f.ID, err = s.store.SaveFile(ctx, &f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetFileItems(ctx, f.ID, []string{item}, 0); err != nil {
		t.Fatal(err)
	}
	other, err := s.store.CreateUser(ctx, "Other", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.UpdateUser(ctx, other.ID, store.UserUpdate{AllowDownloads: &yes}); err != nil {
		t.Fatal(err)
	}
	for token, profile := range map[string]string{"one": u.ID, "two": u.ID, "other": other.ID} {
		if err = s.store.CreateSession(ctx, auth.HashToken(token), profile, "password", ""); err != nil {
			t.Fatal(err)
		}
	}
	m := downloads.New(s.store, s.log)
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log, Downloads: m})
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://arc/api/v1"+path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	body := fmt.Sprintf(`{"item_id":%q,"file_id":%d,"mode":"original","idempotency_key":"operation"}`, item, f.ID)
	if got := request("POST", "/downloads", body, ""); got.Code != 401 {
		t.Fatal("anonymous admitted")
	}
	created := request("POST", "/downloads", body, "one")
	if created.Code != 201 {
		t.Fatalf("create %d %s", created.Code, created.Body)
	}
	var d store.Download
	if err = json.Unmarshal(created.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.State != "preparing" || d.SHA256 != "" {
		t.Fatal("request performed preparation inline")
	}
	for _, secret := range []string{path, auth.HashToken("one"), "operation", "source_path", "session_hash"} {
		if strings.Contains(created.Body.String(), secret) {
			t.Fatalf("response leaked %q", secret)
		}
	}
	replay := request("POST", "/downloads", body, "one")
	if replay.Code != 200 || !strings.Contains(replay.Body.String(), d.ID) {
		t.Fatal("replay not idempotent")
	}
	for _, token := range []string{"two", "other"} {
		for _, method := range []string{"GET", "DELETE"} {
			if w := request(method, "/downloads/"+d.ID, "", token); w.Code != 404 {
				t.Fatalf("cross owner %s %d", method, w.Code)
			}
		}
	}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/downloads", strings.Replace(body, "original", "converted", 1), 422},
		{"POST", "/downloads", strings.Replace(body, fmt.Sprintf(`"file_id":%d`, f.ID), fmt.Sprintf(`"file_id":%d`, f.ID+999), 1), 404},
		{"POST", "/downloads", strings.TrimSuffix(body, "}") + `,"source_path":"anything"}`, 400},
		{"POST", "/downloads", `{"padding":"` + strings.Repeat("x", 5000) + `"}`, 413},
		{"POST", "/downloads", `{"item_id":"` + strings.Repeat("x", 5000) + `"}`, 413},
		{"POST", "/downloads", body + ` {}`, 400},
		{"GET", "/downloads?limit=0", "", 400},
		{"GET", "/downloads?offset=-1", "", 400},
		{"GET", "/downloads?limit=1&limit=2", "", 400},
	} {
		if w := request(tc.method, tc.path, tc.body, "one"); w.Code != tc.status {
			t.Fatalf("%s %s got%d want%d: %s", tc.method, tc.path, w.Code, tc.status, w.Body)
		}
	}
	internal, err := s.store.Download(ctx, u.ID, auth.HashToken("one"), d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Prepare(ctx, internal); err != nil {
		t.Fatal(err)
	}
	ready := request("GET", "/downloads/"+d.ID, "", "one")
	if ready.Code != 200 {
		t.Fatalf("ready %d %s", ready.Code, ready.Body)
	}
	if err = json.Unmarshal(ready.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	if d.State != "ready" || len(d.SHA256) != 64 {
		t.Fatal("digest not published")
	}
	plays, err := s.store.Plays(ctx, store.PlayFilter{UserID: u.ID})
	if err != nil || len(plays) != 0 {
		t.Fatal("preparation recorded playback")
	}
	userdata, err := s.store.UserDataFor(ctx, u.ID, []string{item})
	if err != nil || userdata[item].LastPlayedAt != 0 {
		t.Fatal("preparation changed watch state")
	}
	caps := request("GET", "/capabilities", "", "one")
	if strings.Contains(caps.Body.String(), `"downloads_original"`) {
		t.Fatal("unimplemented transfer advertised")
	}
	if !strings.Contains(caps.Body.String(), `"downloads_preparation"`) {
		t.Fatal("preparation capability missing")
	}
	otherItem, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "episode", Title: "Other episode"}, "shared")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetFileItems(ctx, f.ID, []string{item, otherItem}, 0); err != nil {
		t.Fatal(err)
	}
	if w := request("POST", "/downloads", body, "one"); w.Code != 422 {
		t.Fatal("shared episode file accepted")
	}
	if err = s.store.SetFileItems(ctx, f.ID, []string{item}, 2); err != nil {
		t.Fatal(err)
	}
	if w := request("POST", "/downloads", body, "one"); w.Code != 422 {
		t.Fatal("multipart accepted")
	}
	if err = s.store.SetFileItems(ctx, f.ID, []string{item}, 0); err != nil {
		t.Fatal(err)
	}
	// An external edit before the next scan invalidates the ready descriptor on fetch.
	if err = os.WriteFile(path, []byte("replacement of another size"), 0600); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/downloads/"+d.ID, "", "one"); w.Code != 409 {
		t.Fatalf("stale source got%d", w.Code)
	}
	noLibraries := []int64{}
	if err = s.store.UpdateUser(ctx, u.ID, store.UserUpdate{LibraryIDs: &noLibraries}); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/downloads/"+d.ID, "", "one"); w.Code != 404 {
		t.Fatal("hidden record exposed")
	}
	list := request("GET", "/downloads", "", "one")
	var page struct{ Total int }
	if err = json.Unmarshal(list.Body.Bytes(), &page); err != nil || page.Total != 0 {
		t.Fatal("hidden count leaked")
	}
	no := false
	if err = s.store.UpdateUser(ctx, u.ID, store.UserUpdate{AllowDownloads: &no}); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/downloads", "", "one"); w.Code != 403 {
		t.Fatal("grant revocation ignored")
	}
	if w := request("DELETE", "/downloads/"+d.ID, "", "one"); w.Code != 204 {
		t.Fatal("owner cannot release revoked record")
	}
}

func TestDownloadGrantCanOnlyBeChangedByAdmin(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	admin, err := s.store.CreateUser(ctx, "Admin", true, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for token, profile := range map[string]string{"admin": admin.ID, "viewer": u.ID} {
		if err = s.store.CreateSession(ctx, auth.HashToken(token), profile, "password", ""); err != nil {
			t.Fatal(err)
		}
	}
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	req := func(token, id, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PATCH", "http://arc/api/v1/profiles/"+id, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if got := req("viewer", u.ID, `{"allow_downloads":true}`); got.Code != 403 {
		t.Fatal("viewer granted own download access")
	}
	for _, allowed := range []bool{true, false} {
		w := req("admin", u.ID, fmt.Sprintf(`{"allow_downloads":%v}`, allowed))
		if w.Code != 200 {
			t.Fatalf("grant update %d %s", w.Code, w.Body)
		}
		var dto profileDTO
		if err = json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
			t.Fatal(err)
		}
		if dto.AllowDownloads != allowed {
			t.Fatal("grant DTO inconsistent")
		}
		saved, err := s.store.GetUser(ctx, u.ID)
		if err != nil || saved.AllowDownloads != allowed {
			t.Fatal("grant not persisted")
		}
	}
	if got := req("admin", admin.ID, `{"allow_downloads":false}`); got.Code != 400 {
		t.Fatal("admin effective permission changed")
	}
}

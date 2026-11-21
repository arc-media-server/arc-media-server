package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"ams/internal/auth"
	"ams/internal/store"
)

func TestToolTokenRoutesAndLabelTools(t *testing.T) {
	s, viewer := securityServer(t)
	ctx := t.Context()
	admin, err := s.store.CreateUser(ctx, "Owner", true, "slate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for token, uid := range map[string]string{"admin-fixture": admin.ID, "viewer-fixture": viewer.ID} {
		if err := s.store.CreateSession(ctx, auth.HashToken(token), uid, "password", ""); err != nil {
			t.Fatal(err)
		}
	}
	var logs bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	call := func(method, path, token, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "http://arc"+path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: status %d (want %d): %s", method, strings.SplitN(path, "?", 2)[0], w.Code, want, w.Body.String())
		}
		return w
	}

	call("POST", "/api/v1/devices/tools", "viewer-fixture", `{"name":"Denied"}`, 403)
	call("POST", "/api/v1/devices/tools", "", `{"name":"Denied"}`, 401)
	for _, name := range []string{"", strings.Repeat("a", 101), "line\nbreak"} {
		body, _ := json.Marshal(map[string]string{"name": name})
		call("POST", "/api/v1/devices/tools", "admin-fixture", string(body), 400)
	}
	created := call("POST", "/api/v1/devices/tools", "admin-fixture", `{"name":"Test automation"}`, 201)
	if created.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("credential response cacheable")
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil || result.Token == "" {
		t.Fatal("missing created token")
	}
	token := result.Token
	call("GET", "/api/v1/identity", token, "", 200)
	call("GET", "/api/identity", token, "", 200) // identical device-session checks
	call("GET", "/api/v1/identity?token="+token, "", "", 401)
	listing := call("GET", "/api/v1/devices", "admin-fixture", "", 200)
	if strings.Contains(listing.Body.String(), token) {
		t.Fatal("raw token exposed by device list")
	}
	var devices []struct {
		ID     string `json:"id"`
		Device string `json:"device"`
		IsTool bool   `json:"is_tool"`
	}
	if err := json.Unmarshal(listing.Body.Bytes(), &devices); err != nil {
		t.Fatal(err)
	}
	deviceID := ""
	for _, device := range devices {
		if device.Device == "Test automation" && device.IsTool {
			deviceID = device.ID
		}
	}
	if deviceID == "" {
		t.Fatal("tool not marked in Devices")
	}

	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, title := range []string{"One", "Two", "Three"} {
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		fileID, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: "/fixture/" + title, Size: 1, ModTime: 1}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetFileItems(ctx, fileID, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	mustEdit := func(ids []string, e store.BulkEdit) {
		t.Helper()
		if err := s.store.EditItems(ctx, ids, e); err != nil {
			t.Fatal(err)
		}
	}
	mustEdit(ids, store.BulkEdit{AddLabels: []string{"Keep", "Remove"}})
	body := fmt.Sprintf(`{"item_ids":[%q,%q],"add_labels":["Favourite"],"remove_labels":["remove"],"add_genres":["Drama"]}`, ids[0], ids[1])
	call("POST", "/api/v1/items/bulk", "viewer-fixture", body, 403)
	call("POST", "/api/v1/items/bulk", token, body, 204)
	for _, id := range ids[:2] {
		labels, err := s.store.ItemLabels(ctx, id)
		slices.Sort(labels)
		if err != nil || !slices.Equal(labels, []string{"Favourite", "Keep"}) {
			t.Fatalf("other labels lost: %v %v", labels, err)
		}
		it, err := s.store.GetItem(ctx, id)
		if err != nil || slices.Contains(it.LockedFields, "labels") {
			t.Fatal("labels locked")
		}
	}
	mustEdit(ids[1:2], store.BulkEdit{AddLabels: []string{"Blocked"}})
	mustEdit(ids[2:], store.BulkEdit{AddLabels: []string{"Other"}})
	if err := s.store.UpdateUser(ctx, viewer.ID, store.UserUpdate{ExcludeLabels: &[]string{"blocked"}}); err != nil {
		t.Fatal(err)
	}
	var page itemPage
	w := call("GET", "/api/v1/items?label=favourite&label=other", "viewer-fixture", "", 200)
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("ANY or exclusions failed: %s", w.Body.String())
	}
	for _, it := range page.Items {
		if it.ID == ids[1] {
			t.Fatal("excluded label leaked")
		}
	}
	call("GET", "/api/v1/items/"+ids[1], "viewer-fixture", "", 404)
	facets := call("GET", fmt.Sprintf("/api/v1/libraries/%d/facets", lib), "viewer-fixture", "", 200)
	var f store.LibraryFacets
	if err := json.Unmarshal(facets.Body.Bytes(), &f); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(f.Labels, store.Facet{Value: "Favourite", Count: 1}) || !slices.Contains(f.Labels, store.Facet{Value: "Other", Count: 1}) {
		t.Fatalf("facets not access controlled: %s", facets.Body.String())
	}
	if strings.Contains(facets.Body.String(), "Blocked") {
		t.Fatal("excluded label leaked through facets")
	}
	call("DELETE", "/api/v1/devices/"+deviceID, "admin-fixture", "", 204)
	call("GET", "/api/v1/identity", token, "", 401)
	call("POST", "/api/v1/items/bulk", token, body, 401)
	call("DELETE", "/api/v1/devices/"+deviceID, "admin-fixture", "", 204)
	if strings.Contains(logs.String(), token) || strings.Contains(logs.String(), auth.HashToken(token)) {
		t.Fatal("credential leaked in detailed logs")
	}
}

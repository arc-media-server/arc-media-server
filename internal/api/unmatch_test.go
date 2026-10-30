package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"ams/internal/auth"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
)

func TestUnmatchRoutePermissionsAndExplicitRefresh(t *testing.T) {
	s, viewer := securityServer(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	sc := scanner.New(ctx, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
	defer sc.Wait()
	admin, err := s.store.CreateUser(ctx, "Owner", true, "teal", "", "")
	if err != nil {
		t.Fatal(err)
	}
	lib, err := s.store.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	show, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: "Provider title"}, "show")
	if err != nil {
		t.Fatal(err)
	}
	for token, user := range map[string]string{"owner-token": admin.ID, "viewer-token": viewer.ID} {
		if err = s.store.CreateSession(ctx, auth.HashToken(token), user, "password", "test"); err != nil {
			t.Fatal(err)
		}
	}
	h := New(Deps{Store: s.store, Scanner: sc, Settings: s.settings, Log: s.log})
	request := func(method, path, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://arc/api/v1"+path, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, tc := range []struct {
		token string
		want  int
	}{{"", 401}, {"viewer-token", 403}, {"owner-token", 200}} {
		w := request("POST", "/items/"+show+"/unmatch", tc.token)
		if w.Code != tc.want {
			t.Fatalf("token %s status %d %s", tc.token, w.Code, w.Body.String())
		}
		if w.Code == 200 {
			var item itemDTO
			if err = json.Unmarshal(w.Body.Bytes(), &item); err != nil {
				t.Fatal(err)
			}
			if !item.MatchSuppressed {
				t.Fatal("response omitted suppression")
			}
		}
	}
	if w := request("POST", "/items/"+show+"/refresh", "owner-token"); w.Code != 409 {
		t.Fatalf("suppressed refresh %d", w.Code)
	}
}

package api

import (
	"ams/internal/auth"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type libraryDeletionContext struct {
	context.Context
	once   sync.Once
	remove func()
}

func TestSetMatchReturnsNotFoundIfTitleDisappearsAfterFinalSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, _ := securityServerAt(t, path)
	fixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":603,"title":"Matched fixture","release_date":"2020-01-01"}`)
	}))
	defer fixture.Close()
	transport := fixture.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", fixture.Listener.Addr().String())
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections() })
	sc := scanner.New(t.Context(), s.store, probe.New(""), metadata.NewTMDB("fixture-key"), metadata.NewTVDB(""), s.log)
	defer sc.Wait()
	owner, err := s.store.CreateUser(t.Context(), "Owner", true, "slate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.CreateSession(t.Context(), auth.HashToken("owner"), owner.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	lib, err := s.store.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.ExecContext(t.Context(), `CREATE TABLE fixture_vanished_match(id TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(t.Context(), `INSERT INTO fixture_vanished_match(id) VALUES (?)`, item); err != nil {
		t.Fatal(err)
	}
	// The chosen trace is the scanner's final successful save before the API reload.
	if _, err = db.ExecContext(t.Context(), `CREATE TRIGGER vanish_after_final_match AFTER UPDATE OF match_info ON items
 WHEN NEW.id IN (SELECT id FROM fixture_vanished_match) AND instr(NEW.match_info,'"kind":"chosen"')>0
 BEGIN DELETE FROM items WHERE id=NEW.id; END`); err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Store: s.store, Scanner: sc, Settings: s.settings, Log: s.log})
	r := httptest.NewRequest("PUT", "http://arc/api/v1/items/"+item+"/match", strings.NewReader(`{"provider":"tmdb","id":603}`))
	r.Header.Set("Authorization", "Bearer owner")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatalf("vanished title answered %d instead of 404: %s", w.Code, w.Body.String())
	}
	if current, err := s.store.GetItem(t.Context(), item); err != nil || current != nil {
		t.Fatal("fixture did not remove title at final save", current, err)
	}
}

func (c *libraryDeletionContext) Value(key any) any {
	if key == userKey {
		c.once.Do(c.remove)
	}
	return c.Context.Value(key)
}

func TestSeriesLookupReturnsNotFoundWhenLibraryDisappearsBetweenReads(t *testing.T) {
	s, u := securityServer(t)
	lib, err := s.store.SyncLibrary(t.Context(), "Shows", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	show, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "series", Title: "Fixture"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	parent := context.WithValue(t.Context(), userKey, u)
	ctx := &libraryDeletionContext{Context: parent, remove: func() {
		if err := s.store.DeleteLibrary(t.Context(), lib); err != nil {
			t.Fatal(err)
		}
	}}
	r := httptest.NewRequest("GET", "http://arc/api/series/"+show+"/orderings", nil).WithContext(ctx)
	r.SetPathValue("id", show)
	w := httptest.NewRecorder()
	item, library, ok := s.seriesFromPath(w, r)
	if ok || item != nil || library != nil || w.Code != 404 {
		t.Fatalf("vanished library returned usable lookup: ok=%v item=%v library=%v status=%d", ok, item != nil, library != nil, w.Code)
	}
}

func TestMatchSearchMissingProviderPointsToSettings(t *testing.T) {
	s, u := securityServer(t)
	sc := scanner.New(t.Context(), s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
	defer sc.Wait()
	s.scanner = sc
	lib, err := s.store.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://arc/api/items/"+item+"/match/search?q=fixture", nil)
	r.SetPathValue("id", item)
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	w := httptest.NewRecorder()
	s.searchMatches(w, r)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "Settings") || strings.Contains(w.Body.String(), "ams.yaml") {
		t.Fatal("obsolete provider recovery advice", w.Code, w.Body.String())
	}
}

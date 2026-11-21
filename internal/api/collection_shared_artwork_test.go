package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
)

// A collection made by hand or imported (no TMDB link of its own) whose titles share a TMDB
// collection gets that collection's poster and background when opened, and its artwork editor
// lists that collection's images; artwork chosen by hand is never replaced.
func TestManualCollectionUsesSharedTMDBCollectionArtwork(t *testing.T) {
	var calls atomic.Int64
	fixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if strings.HasSuffix(r.URL.Path, "/images") {
			fmt.Fprint(w, `{"posters":[{"file_path":"/p1.jpg","width":1000,"height":1500}],"backdrops":[{"file_path":"/b1.jpg","width":1920,"height":1080}]}`)
			return
		}
		fmt.Fprint(w, `{"id":777,"name":"Shared","overview":"From TMDB","poster_path":"/shared-poster.jpg","backdrop_path":"/shared-backdrop.jpg"}`)
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

	s, u := securityServer(t)
	s.scanner = scanner.New(t.Context(), s.store, probe.New(""), metadata.NewTMDB("fake-fixture-key"), metadata.NewTVDB(""), s.log)
	defer s.scanner.Wait()
	lib, err := s.store.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i, tmdbColl := range []int64{777, 777, 0} {
		id, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: fmt.Sprint("Movie ", i)}, fmt.Sprint("m", i))
		if err != nil {
			t.Fatal(err)
		}
		file, err := s.store.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: fmt.Sprintf("/fixture/m%d.mkv", i), Size: 1}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.store.SetFileItems(t.Context(), file, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		var ref *store.CollectionRef
		if tmdbColl != 0 {
			ref = &store.CollectionRef{TMDBID: tmdbColl, Name: "Shared"}
		}
		if err = s.store.SetMovieCollection(t.Context(), id, lib, ref); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	manual := &store.Collection{LibraryID: lib, Kind: "manual", Title: "Imported from Plex", Overview: "Mine"}
	if err = s.store.CreateCollection(t.Context(), manual, ids...); err != nil {
		t.Fatal(err)
	}
	if shared, err := s.store.SharedTMDBCollection(t.Context(), manual); err != nil || shared != 777 {
		t.Fatalf("shared TMDB collection = %d, %v; want 777", shared, err)
	}

	open := func(path string, handler func(http.ResponseWriter, *http.Request)) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://arc"+path, nil)
		r.SetPathValue("id", manual.ID)
		r = r.WithContext(context.WithValue(r.Context(), userKey, u))
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	if w := open("/api/collections/"+manual.ID, s.getCollection); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	got, err := s.store.GetCollection(t.Context(), manual.ID)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if got.PosterPath != "/shared-poster.jpg" || got.BackdropPath != "/shared-backdrop.jpg" || got.Overview != "Mine" {
		t.Fatalf("manual collection after open = poster %q backdrop %q overview %q", got.PosterPath, got.BackdropPath, got.Overview)
	}
	w := open("/api/collections/"+manual.ID+"/artwork", s.listCollectionArtwork)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "/p1.jpg") || !strings.Contains(w.Body.String(), "/b1.jpg") {
		t.Fatalf("artwork choices = %d %s", w.Code, w.Body.String())
	}

	// Chosen artwork stays: filling only ever sets what's empty.
	if err = s.store.FillCollectionInfo(t.Context(), manual.ID, "", "/other.jpg", "/other-b.jpg"); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.store.GetCollection(t.Context(), manual.ID); got.PosterPath != "/shared-poster.jpg" {
		t.Fatalf("filled over existing poster: %q", got.PosterPath)
	}

	// An imported collection with no description gets TMDB's.
	bare := &store.Collection{LibraryID: lib, Kind: "manual", Title: "No description"}
	if err = s.store.CreateCollection(t.Context(), bare, ids[0], ids[1]); err != nil {
		t.Fatal(err)
	}
	manual = bare
	if w := open("/api/collections/"+bare.ID, s.getCollection); w.Code != 200 || !strings.Contains(w.Body.String(), "From TMDB") {
		t.Fatalf("bare collection = %d %s", w.Code, w.Body.String())
	}
	if got, _ = s.store.GetCollection(t.Context(), bare.ID); got.Overview != "From TMDB" {
		t.Fatalf("bare collection overview = %q", got.Overview)
	}

	// Titles without a common TMDB collection (only one of three) give none.
	lone := &store.Collection{LibraryID: lib, Kind: "manual", Title: "Mixed"}
	if err = s.store.CreateCollection(t.Context(), lone, ids[0], ids[2]); err != nil {
		t.Fatal(err)
	}
	if shared, err := s.store.SharedTMDBCollection(t.Context(), lone); err != nil || shared != 0 {
		t.Fatalf("mixed collection shared = %d, %v; want 0", shared, err)
	}
}

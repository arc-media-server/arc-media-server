package api

import (
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"context"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestRejectedLibraryPatchLeavesAllFieldsUnchanged(t *testing.T) {
	for _, body := range []string{
		`{"trickplay":true,"collection_grid":"invalid"}`,
		`{"skip_credits":true,"collection_min":99}`,
		`{"trickplay":true,"metadata_language":"invalid!"}`,
		`{"trickplay":true,"rating_region":"invalid"}`,
		`{"trickplay":true,"name":""}`,
		`{"trickplay":true,"name":"Taken"}`,
	} {
		t.Run(body, func(t *testing.T) {
			s, u := securityServer(t)
			root := t.TempDir()
			lib, err := s.store.SyncLibrary(t.Context(), "Fixture", "movies", "tmdb_aired", []string{root})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.store.SyncLibrary(t.Context(), "Taken", "movies", "tmdb_aired", []string{t.TempDir()}); err != nil {
				t.Fatal(err)
			}
			life, cancel := context.WithCancel(t.Context())
			defer cancel()
			s.scanner = scanner.New(life, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
			defer s.scanner.Wait()
			outcomes := 0
			s.scanner.OnFinish = func(int64) { outcomes++ }
			before, err := s.store.GetLibrary(t.Context(), lib)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("PATCH", "/", strings.NewReader(body))
			r.SetPathValue("id", fmt.Sprint(lib))
			r = r.WithContext(context.WithValue(r.Context(), userKey, u))
			w := httptest.NewRecorder()
			s.updateLibrary(w, r)
			if w.Code != 400 && w.Code != 409 {
				t.Fatalf("invalid patch accepted: %d %s", w.Code, w.Body.String())
			}
			after, err := s.store.GetLibrary(t.Context(), lib)
			if err != nil {
				t.Fatal(err)
			}
			if outcomes != 0 {
				t.Error("rejected settings save emitted a scan outcome")
			}
			if !reflect.DeepEqual(before, after) {
				t.Errorf("rejected save partially applied: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestBusyLibraryPatchDoesNotChangeScalarControls(t *testing.T) {
	s, u := securityServer(t)
	id, err := s.store.SyncLibrary(t.Context(), "Busy", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	life, cancel := context.WithCancel(t.Context())
	s.scanner = scanner.New(life, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
	defer func() { cancel(); s.scanner.Wait() }()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- s.scanner.WithLibraryChange(life, id, func(ctx context.Context) error {
			if s.scanner.Start(id) {
				t.Error("scan admitted during settings reservation")
			}
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	before, err := s.store.GetLibrary(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("PATCH", "/", strings.NewReader(`{"trickplay":true,"skip_credits":true}`))
	r.SetPathValue("id", fmt.Sprint(id))
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	w := httptest.NewRecorder()
	s.updateLibrary(w, r)
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if w.Code != 409 {
		t.Fatalf("busy scalar save accepted: %d %s", w.Code, w.Body.String())
	}
	after, err := s.store.GetLibrary(t.Context(), id)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("busy request changed fields: %+v %v", after, err)
	}
}

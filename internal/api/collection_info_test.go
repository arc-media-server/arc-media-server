package api

import (
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEmptyCollectionOverviewDoesNotFetchOnEveryOpen(t *testing.T) {
	for _, status := range []int{200, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int64
			fixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(status)
				fmt.Fprint(w, `{"id":777,"name":"Fixture","overview":""}`)
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
			var logs bytes.Buffer
			s.log = slog.New(slog.NewTextHandler(&logs, nil))
			s.scanner = scanner.New(t.Context(), s.store, probe.New(""), metadata.NewTMDB("fake-fixture-key"), metadata.NewTVDB(""), s.log)
			defer s.scanner.Wait()
			lib, err := s.store.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.store.SetLibraryCollectionMin(t.Context(), lib, 1); err != nil {
				t.Fatal(err)
			}
			movie, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Movie"}, "movie")
			if err != nil {
				t.Fatal(err)
			}
			file, err := s.store.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: "/fixture/movie.mkv", Size: 1}, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.store.SetFileItems(t.Context(), file, []string{movie}, 0); err != nil {
				t.Fatal(err)
			}
			if err = s.store.SetMovieCollection(t.Context(), movie, lib, &store.CollectionRef{TMDBID: 777, Name: "Fixture"}); err != nil {
				t.Fatal(err)
			}
			collections, err := s.store.Collections(t.Context(), lib, store.Access{})
			if err != nil || len(collections) != 1 {
				t.Fatal(collections, err)
			}
			var firstCalls int64
			for i := 0; i < 2; i++ {
				r := httptest.NewRequest("GET", "http://arc/api/collections/"+collections[0].ID, nil)
				r.SetPathValue("id", collections[0].ID)
				r = r.WithContext(context.WithValue(r.Context(), userKey, u))
				w := httptest.NewRecorder()
				s.getCollection(w, r)
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
				if i == 0 {
					firstCalls = calls.Load()
					if firstCalls == 0 {
						t.Fatal("provider fixture was not reached")
					}
				}
			}
			if got := calls.Load(); got != firstCalls {
				t.Fatalf("empty collection overview fetched %d times across two opens", got)
			}
			if status == 503 && strings.Count(logs.String(), "collection metadata lookup failed") != 1 {
				t.Fatal("failure should be diagnosed once", logs.String())
			}
			if strings.Contains(logs.String(), "fake-fixture-key") || strings.Contains(logs.String(), "api_key") || strings.Contains(logs.String(), fixture.URL) {
				t.Fatal("provider secrets/URLs in log", logs.String())
			}
		})
	}
}

func TestCollectionLookupBackoffBoundsAndConcurrency(t *testing.T) {
	now := time.Now()
	cache := collectionLookupCache{now: func() time.Time { return now }}
	active := cache.begin("active")
	if active == nil {
		t.Fatal("first lookup refused")
	}
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if cache.begin("active") != nil {
				t.Error("duplicate lookup admitted")
			}
		})
	}
	wg.Wait()
	cache.finish("active", active, true, false)
	now = now.Add(23 * time.Hour)
	if cache.begin("active") != nil {
		t.Fatal("successful empty result forgotten early")
	}
	now = now.Add(2 * time.Hour)
	retry := cache.begin("active")
	if retry == nil {
		t.Fatal("expired result never retried")
	}
	cache.finish("active", retry, false, false)
	now = now.Add(30 * time.Second)
	if cache.begin("active") != nil {
		t.Fatal("failure repeated before cooldown")
	}
	now = now.Add(31 * time.Second)
	retry = cache.begin("active")
	if retry == nil {
		t.Fatal("failure did not become retryable")
	}
	cache.finish("active", retry, false, true)
	if cache.begin("active") == nil {
		t.Fatal("request cancellation cached as provider failure")
	}
	for i := 0; i < 1024; i++ {
		key := fmt.Sprint(i)
		attempt := cache.begin(key)
		if attempt == nil {
			t.Fatal("completed entries prevented admission")
		}
		cache.finish(key, attempt, true, false)
	}
	if len(cache.attempts) > maxCollectionLookups {
		t.Fatal("unbounded retained lookup hints", len(cache.attempts))
	}
	if cache.begin("active") != nil {
		t.Fatal("active attempt was evicted")
	}
}

package scanner

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ams/internal/activity"
	"ams/internal/metadata"
	"ams/internal/store"
	"strings"
)

func TestMaintenanceRefreshesKnownTitlesWithoutDiscoveringFiles(t *testing.T) {
	for _, kind := range []string{"movies", "shows"} {
		t.Run(kind, func(t *testing.T) {
			entered, release := make(chan struct{}, 1), make(chan struct{})
			localProviders(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/3/movie/101":
					entered <- struct{}{}
					<-release
					fmt.Fprint(w, `{"id":101,"title":"Provider title","release_date":"2020-01-01","overview":"Fresh details"}`)
				case "/3/tv/303":
					if r.URL.Query().Get("append_to_response") == "season/1" {
						fmt.Fprint(w, `{"season/1":{"episodes":[{"id":100,"name":"Pilot","season_number":1,"episode_number":1,"air_date":"2020-01-01"}]}}`)
					} else {
						entered <- struct{}{}
						<-release
						fmt.Fprint(w, `{"id":303,"name":"Provider title","first_air_date":"2020-01-01","overview":"Fresh details","seasons":[{"season_number":1}]}`)
					}
				default:
					t.Errorf("unexpected provider path: %s", r.URL.Path)
					http.Error(w, "unexpected", 500)
				}
			})
			st, s, lib, root := libraryFixture(t, kind, map[string]string{"Undiscovered/Undiscovered S01E01.mkv": "fixture"})
			s.Activity = activity.New()
			ctx := context.Background()
			itemKind, tmdb := "movie", int64(101)
			if kind == "shows" {
				itemKind, tmdb = "series", 303
			}
			id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: itemKind, Title: "Known title", TMDBID: tmdb}, "known")
			if err != nil {
				t.Fatal(err)
			}
			if err := st.SetMetadataState(ctx, id, "matched"); err != nil {
				t.Fatal(err)
			}
			title := "My locked title"
			if err := st.EditItem(ctx, id, store.ItemEdit{Title: &title}); err != nil {
				t.Fatal(err)
			}
			s.tmdb = metadata.NewTMDB("local-fixture")
			if !s.Maintain(lib) {
				t.Fatal("maintenance not admitted")
			}
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("known title not refreshed")
			}
			running, _ := s.Activity.Snapshot()
			if len(running) != 1 || len(running[0].Current) != 1 || !strings.Contains(running[0].Current[0].Subject, title) || !strings.Contains(running[0].Current[0].Detail, "TMDB") {
				close(release)
				s.Wait()
				t.Fatalf("held refresh activity: %+v", running)
			}
			if s.Maintain(lib) || s.Start(lib) {
				t.Fatal("concurrent library work admitted")
			}
			close(release)
			s.Wait()
			it, err := st.GetItem(ctx, id)
			if err != nil || it == nil || it.Title != title || it.Overview != "Fresh details" {
				t.Fatalf("metadata/lock: %+v %v", it, err)
			}
			synced, err := st.SyncedAt(ctx, id)
			if err != nil || synced == 0 {
				t.Fatalf("not refreshed: %d %v", synced, err)
			}
			items, total, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: itemKind})
			if err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("unexpected discovery: %v %d %v", items, total, err)
			}
			file, err := st.GetFileByPath(ctx, filepath.Join(root, "Undiscovered", "Undiscovered S01E01.mkv"))
			if err != nil || file == nil || file.Missing {
				t.Fatalf("media file was visited: %+v %v", file, err)
			}
		})
	}
}

func TestQueuedDiscoveryPreemptsSameLibraryMetadata(t *testing.T) {
	entered, cancelled, release := make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{})
	localProviders(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/3/movie/101" {
			select {
			case entered <- struct{}{}:
			default:
			}
			select {
			case <-r.Context().Done():
				select {
				case cancelled <- struct{}{}:
				default:
				}
				return
			case <-release:
			}
			fmt.Fprint(w, `{"id":101,"title":"Known","release_date":"2020-01-01"}`)
			return
		}
		fmt.Fprint(w, `{"results":[]}`)
	})
	st, s, lib, _ := libraryFixture(t, "movies", map[string]string{"New Discovery (2020).mkv": "isolated"})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() { unblock(); s.Wait() })
	ctx := context.Background()
	id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Known", TMDBID: 101}, "known")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetMetadataState(ctx, id, "matched"); err != nil {
		t.Fatal(err)
	}
	s.tmdb = metadata.NewTMDB("local-fixture")
	s.MaintenanceAllowed = func() bool { return !s.DiscoveryPending() }
	if !s.Maintain(lib) {
		t.Fatal("maintenance denied")
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("maintenance did not reach provider")
	}
	s.Request(lib)
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("queued discovery did not cancel same-library maintenance")
	}
	unblock()
	s.Wait()
	items, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "movie"})
	if err != nil || len(items) == 0 {
		t.Fatalf("queued scan never discovered media: %+v %v", items, err)
	}
}

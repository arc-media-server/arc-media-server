package scanner

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"ams/internal/metadata"
	"ams/internal/schedule"
	"ams/internal/store"
)

func fixtureSQL(t *testing.T, root, query string, args ...any) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(filepath.Dir(root), "ams.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func TestUnchangedAgedRescanDefersMetadataToMaintenance(t *testing.T) {
	for _, kind := range []string{"movies", "shows"} {
		t.Run(kind, func(t *testing.T) {
			var requests atomic.Int32
			localProviders(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/3/movie/101":
					fmt.Fprint(w, `{"id":101,"title":"Example","release_date":"2020-01-01","overview":"Fresh"}`)
				case "/3/tv/303":
					if r.URL.Query().Get("append_to_response") == "season/1" {
						fmt.Fprint(w, `{"season/1":{"episodes":[{"id":100,"name":"Pilot","season_number":1,"episode_number":1,"air_date":"2020-01-01"}]}}`)
					} else {
						fmt.Fprint(w, `{"id":303,"name":"Example","first_air_date":"2020-01-01","overview":"Fresh","seasons":[{"season_number":1}]}`)
					}
				default:
					t.Errorf("unexpected provider %s", r.URL)
					http.Error(w, "unexpected", 404)
				}
			})
			file := "Example (2020) {tmdb-101}/Example (2020) {tmdb-101}.mkv"
			itemKind := "movie"
			if kind == "shows" {
				file = "Example (2020) {tmdb-303}/Example S01E01.mkv"
				itemKind = "series"
			}
			st, s, lib, root := libraryFixture(t, kind, map[string]string{file: "seed"})
			s.tmdb = metadata.NewTMDB("fixture")
			s.Start(lib)
			s.Wait()
			if status := s.Status(lib); status.Error != "" {
				t.Fatal(status.Error)
			}
			items, _, err := st.ListItems(context.Background(), store.ItemQuery{LibraryID: lib, Kind: itemKind})
			if err != nil || len(items) != 1 {
				t.Fatalf("items %v %v", items, err)
			}
			id := items[0].ID
			fixtureSQL(t, root, `UPDATE items SET synced_at=?,next_aired='2020-01-01' WHERE id=?`, time.Now().Add(-45*24*time.Hour).Unix(), id)
			before := requests.Load()
			for range 2 {
				s.Start(lib)
				s.Wait()
				if status := s.Status(lib); status.Error != "" || status.FilesProbed != 0 || status.FilesReused != 1 {
					t.Fatalf("rescan %+v", status)
				}
			}
			if requests.Load() != before {
				t.Fatalf("unchanged stale scan made %d provider calls", requests.Load()-before)
			}
			var allowed atomic.Bool
			s.MaintenanceAllowed = allowed.Load
			s.Maintain(lib)
			s.Wait()
			if requests.Load() != before {
				t.Fatal("disabled maintenance made provider calls")
			}
			hour := time.Now().Hour()
			s.Window = schedule.New((hour+6)%24, (hour+7)%24)
			if s.Window.Open() {
				t.Fatal("fixture window unexpectedly open")
			}
			allowed.Store(true) // Models Run now admission outside the window.
			s.Maintain(lib)
			s.Wait()
			if status := s.Status(lib); status.Error != "" {
				t.Fatal(status.Error)
			}
			if requests.Load() <= before {
				t.Fatal("admitted maintenance did not refresh aged metadata")
			}
			synced, err := st.SyncedAt(context.Background(), id)
			if err != nil || synced < time.Now().Add(-time.Minute).Unix() {
				t.Fatalf("not maintained %d %v", synced, err)
			}
			before = requests.Load()
			if err := st.SetLibraryLanguage(context.Background(), lib, "fr"); err != nil {
				t.Fatal(err)
			}
			s.Start(lib)
			s.Wait()
			if requests.Load() <= before {
				t.Fatal("explicit language change was treated as unchanged")
			}
		})
	}
}

func TestNewUnknownEpisodeFetchesOnceAndExistingUnknownWaits(t *testing.T) {
	var requests atomic.Int32
	var available atomic.Bool
	localProviders(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("append_to_response") == "season/1" {
			if available.Load() {
				fmt.Fprint(w, `{"season/1":{"episodes":[{"id":100,"name":"Pilot","season_number":1,"episode_number":1,"air_date":"2020-01-01"},{"id":101,"name":"Second","season_number":1,"episode_number":2,"air_date":"2020-01-02"}]}}`)
			} else {
				fmt.Fprint(w, `{"season/1":{"episodes":[{"id":100,"name":"Pilot","season_number":1,"episode_number":1,"air_date":"2020-01-01"}]}}`)
			}
		} else {
			fmt.Fprint(w, `{"id":303,"name":"Example","first_air_date":"2020-01-01","seasons":[{"season_number":1}]}`)
		}
	})
	st, s, lib, root := libraryFixture(t, "shows", map[string]string{"Example (2020) {tmdb-303}/Example S01E01.mkv": "seed"})
	s.tmdb = metadata.NewTMDB("fixture")
	s.Start(lib)
	s.Wait()
	before := requests.Load()
	path := filepath.Join(root, "Example (2020) {tmdb-303}", "Example S01E02.mkv")
	if err := os.WriteFile(path, []byte("new episode"), 0644); err != nil {
		t.Fatal(err)
	}
	// Pre-seed inspection only, so this test isolates metadata and identity work.
	info, _ := os.Stat(path)
	if _, err := st.SaveFile(context.Background(), &store.MediaFile{LibraryID: lib, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix()}, 1, 1); err != nil {
		t.Fatal(err)
	}
	s.Start(lib)
	s.Wait()
	if requests.Load() <= before {
		t.Fatal("new unknown episode did not request current metadata")
	}
	before = requests.Load()
	s.Start(lib)
	s.Wait()
	if requests.Load() != before {
		t.Fatal("existing unknown episode repeatedly requested providers")
	}
	shows, _, _ := st.ListItems(context.Background(), store.ItemQuery{LibraryID: lib, Kind: "series"})
	kids, err := st.Children(context.Background(), shows[0].ID)
	if err != nil || len(kids) != 2 {
		t.Fatalf("unknown episode disappeared %+v %v", kids, err)
	}
	ctx := context.Background()
	unknownID := ""
	for _, kid := range kids {
		if kid.MetadataState == "unmatched" {
			unknownID = kid.ID
		}
	}
	if unknownID == "" {
		t.Fatal("missing file-only episode")
	}
	viewer, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReportProgress(ctx, viewer.ID, unknownID, 70000, 120000); err != nil {
		t.Fatal(err)
	}
	available.Store(true)
	fixtureSQL(t, root, `UPDATE items SET synced_at=?,next_aired='' WHERE id=?`, time.Now().Add(-2*time.Hour).Unix(), shows[0].ID)
	before = requests.Load()
	s.Maintain(lib)
	s.Wait()
	if requests.Load() <= before {
		t.Fatal("maintenance ignored existing unknown episode")
	}
	episode, err := st.GetItem(ctx, unknownID)
	if err != nil || episode == nil || episode.MetadataState != "matched" || episode.Title != "Second" {
		t.Fatalf("episode identity lost: %+v %v", episode, err)
	}
	state, err := st.UserDataFor(ctx, viewer.ID, []string{unknownID})
	if err != nil || state[unknownID].PositionMS != 70000 {
		t.Fatalf("watch history lost: %+v %v", state, err)
	}

}

func TestOldUnmatchedRetriesOnlyInMaintenance(t *testing.T) {
	for _, kind := range []string{"movies", "shows"} {
		t.Run(kind, func(t *testing.T) {
			var requests atomic.Int32
			localProviders(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"results":[]}`)
			})
			file, itemKind := "Unknown (2020).mkv", "movie"
			if kind == "shows" {
				file = "Unknown (2020)/Unknown S01E01.mkv"
				itemKind = "series"
			}
			st, s, lib, root := libraryFixture(t, kind, map[string]string{file: "fixture"})
			s.tmdb = metadata.NewTMDB("fixture")
			s.Start(lib)
			s.Wait()
			items, _, err := st.ListItems(context.Background(), store.ItemQuery{LibraryID: lib, Kind: itemKind})
			if err != nil || len(items) != 1 || items[0].MetadataState != "unmatched" {
				t.Fatalf("unmatched %+v %v", items, err)
			}
			fixtureSQL(t, root, `UPDATE items SET updated_at=? WHERE id=?`, time.Now().Add(-48*time.Hour).Unix(), items[0].ID)
			before := requests.Load()
			s.Start(lib)
			s.Wait()
			if requests.Load() != before {
				t.Fatal("ordinary scan retried an old unmatched title")
			}
			s.Maintain(lib)
			s.Wait()
			if requests.Load() <= before {
				t.Fatal("maintenance did not retry old unmatched title")
			}
			before = requests.Load()
			s.Start(lib)
			s.Wait()
			if requests.Load() != before {
				t.Fatal("scan repeated the maintenance no-match lookup")
			}
		})
	}
}

func TestMetadataMaintenanceDrainsFailedPagesWithoutDiscoveryRetry(t *testing.T) {
	_, s, lib, root := libraryFixture(t, "movies", map[string]string{"Undiscovered.mkv": "fixture"})
	// Use no configured provider to fail every attempt immediately. The private
	// maintenance pass must still traverse every due ID once, beyond its 200-row page.
	for n := range 205 {
		fixtureSQL(t, root, `INSERT INTO items (id,library_id,kind,match_key,title,sort_title,metadata_state,added_at,updated_at) VALUES (?,?,'movie',?,?,?,'matched',1,1)`, fmt.Sprintf("movie-%03d", n), lib, fmt.Sprint(n), fmt.Sprint(n), fmt.Sprint(n))
	}
	if err := s.refreshMovies(context.Background(), lib); err != nil {
		t.Fatal(err)
	}
	status := s.Status(lib)
	if status.Done != 205 || status.MetadataErrors != 205 || status.FilesSeen != 0 || s.retryPending[lib] {
		t.Fatalf("maintenance stopped/repeated/discovered: %+v", status)
	}
}

func TestMetadataLookupFailureDoesNotScheduleDiscovery(t *testing.T) {
	var requests atomic.Int32
	localProviders(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "fixture rejected", http.StatusUnauthorized)
	})
	st, s, lib, _ := libraryFixture(t, "movies", map[string]string{"Undiscovered.mkv": "fixture"})
	if _, _, err := st.EnsureItem(context.Background(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Known", TMDBID: 101}, "known"); err != nil {
		t.Fatal(err)
	}
	s.tmdb = metadata.NewTMDB("fixture")
	s.retryBase = time.Hour
	s.Maintain(lib)
	s.Wait()
	if requests.Load() != 1 || s.Status(lib).MetadataErrors != 1 || s.Status(lib).FilesSeen != 0 || s.retryPending[lib] {
		t.Fatalf("maintenance queued discovery: %+v retries=%v calls=%d", s.Status(lib), s.retryPending, requests.Load())
	}
}

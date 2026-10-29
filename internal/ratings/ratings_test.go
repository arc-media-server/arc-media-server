package ratings

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"ams/internal/store"
)

func TestOMDbScores(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("i") {
		case "tt0111161":
			w.Write([]byte(`{"Response":"True","Ratings":[{"Source":"Internet Movie Database","Value":"9.3/10"},
				{"Source":"Rotten Tomatoes","Value":"89%"},{"Source":"Metacritic","Value":"82/100"}]}`))
		case "tt0000001":
			w.Write([]byte(`{"Response":"True","Ratings":[]}`))
		case "limit":
			w.Write([]byte(`{"Response":"False","Error":"Request limit reached!"}`))
		default:
			w.Write([]byte(`{"Response":"False","Error":"Invalid API key!"}`))
		}
	}))
	defer srv.Close()
	t.Setenv("AMS_OMDB_URL", srv.URL)
	ctx := context.Background()

	rt, mc, err := omdbScores(ctx, srv.Client(), "key", "tt0111161")
	if err != nil || rt == nil || *rt != 89 || mc == nil || *mc != 82 {
		t.Errorf("scores = %v, %v, %v", rt, mc, err)
	}
	if rt, mc, err := omdbScores(ctx, srv.Client(), "key", "tt0000001"); err != nil || rt != nil || mc != nil {
		t.Errorf("no scores = %v, %v, %v", rt, mc, err)
	}
	if _, _, err := omdbScores(ctx, srv.Client(), "key", "limit"); !errors.Is(err, errOMDbLimit) {
		t.Errorf("limit: %v", err)
	}
	if _, _, err := omdbScores(ctx, srv.Client(), "bad", "other"); !errors.Is(err, errOMDbKey) {
		t.Errorf("bad key: %v", err)
	}
}

// Exercise the manager entry point with both a missing dataset and an existing cache:
// neither may contact IMDb or overwrite a title's saved score while ingestion is paused.
func TestPassDoesNotDownloadOrApplyIMDb(t *testing.T) {
	for name, cached := range map[string]bool{"missing": false, "cached": true} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			st, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"), store.Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			lib, err := st.CreateLibrary(ctx, "Test", "movies", "tmdb_aired", []string{"/fixture"})
			if err != nil {
				t.Fatal(err)
			}
			id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture", IMDBID: "tt123"}, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			fid, err := st.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: "/fixture/movie.mkv", Size: 1, ModTime: 1}, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
				t.Fatal(err)
			}
			if err := st.SetIMDbRatings(ctx, map[string]store.IMDbRating{"tt123": {Rating: 6, Votes: 10}}); err != nil {
				t.Fatal(err)
			}
			var requests, criticsRequests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/imdb" {
					requests.Add(1)
					w.WriteHeader(503)
					return
				}
				criticsRequests.Add(1)
				w.Write([]byte(`{"Response":"True","Ratings":[{"Source":"Rotten Tomatoes","Value":"89%"},{"Source":"Metacritic","Value":"82/100"}]}`))
			}))
			defer srv.Close()
			t.Setenv("AMS_IMDB_RATINGS_URL", srv.URL+"/imdb")
			t.Setenv("AMS_OMDB_URL", srv.URL)
			m := New(t.TempDir(), st, slog.New(slog.NewTextHandler(io.Discard, nil)))
			m.SetOMDbKey("fixture-key")
			if cached {
				f, err := os.Create(m.file())
				if err != nil {
					t.Fatal(err)
				}
				z := gzip.NewWriter(f)
				if _, err := io.WriteString(z, "tconst\taverageRating\tnumVotes\ntt123\t9.9\t999\n"); err != nil {
					t.Fatal(err)
				}
				if err := z.Close(); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			}
			m.pass(ctx)
			if n := requests.Load(); n != 0 {
				t.Fatalf("IMDb requests = %d, want none", n)
			}
			item, err := st.GetItem(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if item.IMDbRating != 6 || item.IMDbVotes != 10 || item.IMDBID != "tt123" {
				t.Fatalf("IMDb data changed: %+v", item)
			}
			if criticsRequests.Load() != 1 || item.RottenTomatoes == nil || *item.RottenTomatoes != 89 || item.Metacritic == nil || *item.Metacritic != 82 {
				t.Fatalf("optional critic scores not updated: %+v", item)
			}
			if !cached {
				if _, err := os.Stat(m.file()); !os.IsNotExist(err) {
					t.Fatalf("unexpected cache result: %v", err)
				}
			}
		})
	}
}

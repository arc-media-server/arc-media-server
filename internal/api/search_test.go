package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"ams/internal/auth"
	"ams/internal/store"
)

// Search scores names only and loads details for the titles shown: the results must still honour
// typos, other names, a profile's hidden labels, titles whose files are gone, and the busiest
// person first among equal matches.
func TestSearchResults(t *testing.T) {
	s, viewer := securityServer(t)
	ctx := context.Background()
	lib, err := s.store.SyncLibrary(ctx, "Search", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, title := range []string{"The Matrix", "Spirited Away", "Hidden Picture", "Gone Film", "Matrimony"} {
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		ids[title] = id
		if title == "Gone Film" {
			continue // no file: not playable
		}
		fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: filepath.Join(t.TempDir(), title)}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.store.SetAltTitlesKeywords(ctx, ids["Spirited Away"], []string{"Sen to Chihiro no Kamikakushi"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.store.EditItems(ctx, []string{ids["Hidden Picture"]}, store.BulkEdit{AddLabels: []string{"grown-ups"}}); err != nil {
		t.Fatal(err)
	}
	labels := []string{"grown-ups"}
	if err := s.store.UpdateUser(ctx, viewer.ID, store.UserUpdate{ExcludeLabels: &labels}); err != nil {
		t.Fatal(err)
	}
	// Keanu is in two present films; Keanu Lookalike in one; Keanu Ghost only in the one without files.
	credit := func(id int64, name string) store.Credit { return store.Credit{TMDBID: id, Name: name, Role: "actor"} }
	for title, credits := range map[string][]store.Credit{
		"The Matrix":    {credit(1, "Keanu Reeves"), credit(2, "Keanu Lookalike")},
		"Matrimony":     {credit(1, "Keanu Reeves")},
		"Gone Film":     {credit(3, "Keanu Ghost")},
		"Spirited Away": {credit(4, "Rumi Hiiragi")},
	} {
		if err := s.store.SetItemDetails(ctx, ids[title], nil, credits); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.store.CreateSession(ctx, auth.HashToken("search-viewer"), viewer.ID, "password", "fixture"); err != nil {
		t.Fatal(err)
	}
	find := func(q string) (movies, people []string) {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/search?q="+url.QueryEscape(q), nil)
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "search-viewer"})
		w := httptest.NewRecorder()
		s.signedIn(s.searchAll)(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", q, w.Code, w.Body.String())
		}
		var res struct {
			Movies []struct {
				Title     string
				LibraryID int64 `json:"library_id"`
			}
			People []struct{ Name string }
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		for _, m := range res.Movies {
			if m.LibraryID == 0 {
				t.Fatalf("%s: %s came back without its details", q, m.Title)
			}
			movies = append(movies, m.Title)
		}
		for _, p := range res.People {
			people = append(people, p.Name)
		}
		return movies, people
	}
	for _, tc := range []struct{ q, want string }{
		{"matirx", "[The Matrix]"},          // a typo
		{"matri", "[Matrimony The Matrix]"}, // a leading match first, then one after "The"
		{"chihiro", "[Spirited Away]"},      // another name
		{"hidden", "[]"},                    // a label the profile doesn't see
		{"gone", "[]"},                      // no files left
	} {
		if movies, _ := find(tc.q); fmtList(movies) != tc.want {
			t.Errorf("%q: movies %v, want %s", tc.q, movies, tc.want)
		}
	}
	if _, people := find("keanu"); fmtList(people) != "[Keanu Reeves Keanu Lookalike]" {
		t.Errorf("keanu: people %v, want the busiest first and no one only in titles without files", people)
	}
	// People added after a search are found by the next one.
	if err := s.store.SetItemDetails(ctx, ids["Spirited Away"], nil, []store.Credit{credit(4, "Rumi Hiiragi"), credit(5, "Miyu Irino")}); err != nil {
		t.Fatal(err)
	}
	if _, people := find("miyu"); fmtList(people) != "[Miyu Irino]" {
		t.Errorf("miyu: people %v, want the newly credited person", people)
	}
}

// Shortlisting by name still applies profile visibility and the minimum collection size.
func TestSearchCollectionMatches(t *testing.T) {
	s, viewer := securityServer(t)
	ctx := context.Background()
	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	addMovie := func(title string) string {
		t.Helper()
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		file, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: filepath.Join(t.TempDir(), title)}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetFileItems(ctx, file, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		return id
	}
	visible, hidden := addMovie("First"), addMovie("Second")
	for _, id := range []string{visible, hidden} {
		if err := s.store.SetMovieCollection(ctx, id, lib, &store.CollectionRef{TMDBID: 1, Name: "Café Collection"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"Café Favourites", "Summer Favourites"} {
		c := &store.Collection{LibraryID: lib, Kind: "manual", Title: name}
		if err := s.store.CreateCollection(ctx, c); err != nil {
			t.Fatal(err)
		}
		if err := s.store.AddToCollection(ctx, c.ID, []string{visible, hidden}); err != nil {
			t.Fatal(err)
		}
	}
	labels := []string{"hidden"}
	if err := s.store.EditItems(ctx, []string{hidden}, store.BulkEdit{AddLabels: labels}); err != nil {
		t.Fatal(err)
	}
	if err := s.store.UpdateUser(ctx, viewer.ID, store.UserUpdate{ExcludeLabels: &labels}); err != nil {
		t.Fatal(err)
	}
	if err := s.store.CreateSession(ctx, auth.HashToken("search-collections-viewer"), viewer.ID, "password", "fixture"); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/v1/search?q=cafe", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "search-collections-viewer"})
	w := httptest.NewRecorder()
	s.signedIn(s.searchAll)(w, r)
	var res struct{ Collections []collectionDTO }
	if w.Code != http.StatusOK {
		t.Fatalf("search: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Collections) != 1 || res.Collections[0].Title != "Café Favourites" || res.Collections[0].Count != 1 {
		t.Fatalf("collections: %+v", res.Collections)
	}
}

func fmtList(v []string) string {
	b, _ := json.Marshal(v)
	out := string(b)
	if v == nil {
		out = "[]"
	}
	// ["a","b"] -> [a b], to compare readably.
	r := []rune{}
	for _, c := range out {
		switch c {
		case '"':
		case ',':
			r = append(r, ' ')
		default:
			r = append(r, c)
		}
	}
	return string(r)
}

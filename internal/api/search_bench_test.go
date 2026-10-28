package api

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"ams/internal/auth"
	"ams/internal/store"
)

// searchLibrary fills a server with a library about the size of a large home one: 2,000 movies
// and 400 shows (8,000 episodes), 40 credits each from 60,000 people, and two other names each.
func searchLibrary(b *testing.B) *Server {
	b.Helper()
	s, viewer := securityServer(b)
	ctx := context.Background()
	lib, err := s.store.SyncLibrary(ctx, "Bench", "movies", "tmdb_aired", []string{b.TempDir()})
	if err != nil {
		b.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(1, 2))
	words := strings.Fields("star night dark city river moon king queen house storm last first lost secret blue red winter summer black white silent golden iron glass shadow road heart fire ice")
	name := func(n int) string {
		var p []string
		for range n {
			p = append(p, words[rng.IntN(len(words))])
		}
		return strings.Join(p, " ")
	}
	people := make([]store.Credit, 60000)
	for i := range people {
		people[i] = store.Credit{TMDBID: int64(i + 1), Name: fmt.Sprintf("%s %s%d", words[rng.IntN(len(words))], words[rng.IntN(len(words))], i), Role: "actor"}
	}
	var ids []string
	add := func(kind, title, parent string) string {
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: kind, Title: title, ParentID: parent, Overview: strings.Repeat("A long description of the story. ", 12)}, title+parent)
		if err != nil {
			b.Fatal(err)
		}
		ids = append(ids, id)
		return id
	}
	for i := range 2400 {
		kind := "movie"
		if i >= 2000 {
			kind = "series"
		}
		id := add(kind, fmt.Sprintf("The %s %d", name(2), i), "")
		credits := make([]store.Credit, 40)
		for j := range credits {
			credits[j] = people[rng.IntN(len(people))]
			credits[j].Order = j
		}
		if err := s.store.SetItemDetails(ctx, id, []string{"Drama"}, credits); err != nil {
			b.Fatal(err)
		}
		if err := s.store.SetAltTitlesKeywords(ctx, id, []string{name(3), name(2)}, nil); err != nil {
			b.Fatal(err)
		}
		if kind == "series" {
			for e := range 20 {
				add("episode", fmt.Sprintf("%s %d", name(2), e), id)
			}
		}
	}
	fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: filepath.Join(b.TempDir(), "f.mkv")}, 1, 1)
	if err != nil {
		b.Fatal(err)
	}
	if err := s.store.SetFileItems(ctx, fid, ids, 0); err != nil {
		b.Fatal(err)
	}
	if err := s.store.CreateSession(ctx, auth.HashToken("bench"), viewer.ID, "password", "bench"); err != nil {
		b.Fatal(err)
	}
	return s
}

func BenchmarkSearch(b *testing.B) {
	s := searchLibrary(b)
	for _, q := range []string{"st", "storm", "shadw road", "the golden king"} {
		b.Run(q, func(b *testing.B) {
			for b.Loop() {
				r := httptest.NewRequest("GET", "/api/search?q="+url.QueryEscape(q), nil)
				r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "bench"})
				w := httptest.NewRecorder()
				s.signedIn(s.searchAll)(w, r)
				if w.Code != 200 {
					b.Fatal(w.Body.String())
				}
			}
		})
	}
}

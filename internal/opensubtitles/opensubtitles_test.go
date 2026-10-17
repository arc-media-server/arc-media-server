package opensubtitles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A stand-in for the API, answering as its documentation describes.
func fakeAPI(t *testing.T) *httptest.Server {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The download link itself is a plain URL; everything else needs the key.
		if !strings.HasPrefix(r.URL.Path, "/file/") && (r.Header.Get("Api-Key") != "good" || r.Header.Get("User-Agent") == "") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"message":"Error, invalid API key"}`))
			return
		}
		switch {
		case r.URL.Path == "/infos/formats":
			w.Write([]byte(`{"data":{"output_formats":["srt"]}}`))
		case r.URL.Path == "/login":
			var b map[string]string
			json.NewDecoder(r.Body).Decode(&b)
			if b["password"] != "secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Write([]byte(`{"token":"tok123","base_url":"api.opensubtitles.com"}`))
		case r.URL.Path == "/subtitles":
			q := r.URL.Query()
			if q.Get("parent_tmdb_id") != "1437" || q.Get("season_number") != "1" || q.Get("episode_number") != "2" || q.Get("languages") != "en,fr" {
				t.Errorf("search query: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"total_count":3,"data":[
				{"id":"1","attributes":{"language":"en","download_count":900,"release":"Firefly.S01E02.720p","ai_translated":true,"files":[{"file_id":11,"file_name":"a.srt"}]}},
				{"id":"2","attributes":{"language":"en","download_count":50,"release":"Firefly.S01E02.DVDRip","moviehash_match":true,"files":[{"file_id":22,"file_name":"b.srt"}]}},
				{"id":"3","attributes":{"language":"fr","download_count":300,"hearing_impaired":true,"release":"Firefly.S01E02","files":[{"file_id":33,"file_name":"c.srt"}]}}]}`))
		case r.URL.Path == "/download":
			if r.Header.Get("Authorization") != "Bearer tok123" {
				t.Errorf("download without the account token")
			}
			w.Write([]byte(`{"link":"` + srv.URL + `/file/22","file_name":"b.srt","remaining":19}`))
		case strings.HasPrefix(r.URL.Path, "/file/"):
			w.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClient(t *testing.T) {
	srv := fakeAPI(t)
	ctx := context.Background()
	c := New("Arc test")
	c.base = srv.URL

	if _, err := c.Search(ctx, Query{}); !errors.Is(err, ErrNoKey) {
		t.Errorf("no key: %v", err)
	}
	c.SetCredentials("bad", "", "")
	if err := c.Check(ctx); err == nil || !strings.Contains(err.Error(), "didn't accept") {
		t.Errorf("bad key: %v", err)
	}
	c.SetCredentials("good", "me", "wrong")
	if err := c.Check(ctx); err == nil || !strings.Contains(err.Error(), "sign in") {
		t.Errorf("bad password: %v", err)
	}
	c.SetCredentials("good", "me", "secret")
	if err := c.Check(ctx); err != nil {
		t.Fatal(err)
	}

	res, err := c.Search(ctx, Query{ParentTMDBID: 1437, Season: 1, Episode: 2, Languages: []string{"en", "FR"}})
	if err != nil {
		t.Fatal(err)
	}
	// The hash match first, then human translations by downloads; AI translation last.
	if len(res) != 3 || res[0].FileID != 22 || res[1].FileID != 33 || res[2].FileID != 11 || !res[2].MachineTranslated || !res[1].HearingImpaired {
		t.Errorf("results: %+v", res)
	}
	data, name, remaining, err := c.Download(ctx, 22)
	if err != nil {
		t.Fatal(err)
	}
	if name != "b.srt" || remaining != 19 || !strings.Contains(string(data), "Hi") {
		t.Errorf("download: %q %q %d", data, name, remaining)
	}
}

func TestHash(t *testing.T) {
	// A file of 131072 zero bytes hashes to its size.
	p := filepath.Join(t.TempDir(), "zero.bin")
	os.WriteFile(p, make([]byte, 131072), 0o644)
	if h, err := Hash(p); err != nil || h != "0000000000020000" {
		t.Errorf("hash = %q, %v", h, err)
	}
	small := filepath.Join(t.TempDir(), "small.bin")
	os.WriteFile(small, []byte("x"), 0o644)
	if _, err := Hash(small); err == nil {
		t.Error("tiny file hashed")
	}
}

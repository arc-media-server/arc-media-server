package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ams/internal/optimize"
	"ams/internal/store"
)

func TestOptimizedStreamGuardsAndRepair(t *testing.T) {
	s, viewer := securityServer(t)
	ctx := t.Context()
	root := t.TempDir()
	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{root})
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "source.mkv")
	if err := os.WriteFile(source, []byte("original source"), 0600); err != nil {
		t.Fatal(err)
	}
	modified := time.Unix(time.Now().Unix()-10, 0)
	if err := os.Chtimes(source, modified, modified); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	f := &store.MediaFile{LibraryID: lib, Path: source, Size: info.Size(), ModTime: info.ModTime().Unix(), Container: "matroska"}
	f.ID, err = s.store.SaveFile(ctx, f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetFileItems(ctx, f.ID, []string{item}, 0); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "optimized")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	s.optimizer = optimize.New(dir, s.store, s.log)
	request := func(method, url, body string) *http.Request {
		r := httptest.NewRequest(method, url, strings.NewReader(body))
		r.SetPathValue("id", item)
		return r.WithContext(context.WithValue(r.Context(), userKey, viewer))
	}
	makeCopy := func(o *store.Optimized, content string) store.Optimized {
		t.Helper()
		if o == nil {
			if n, err := s.store.QueueOptimize(ctx, []int64{f.ID}, 720); err != nil || n != 1 {
				t.Fatalf("queue %d %v", n, err)
			}
			o, err = s.store.NextOptimize(ctx)
			if err != nil || o == nil {
				t.Fatalf("next %+v %v", o, err)
			}
		}
		info, err := os.Stat(source)
		if err != nil {
			t.Fatal(err)
		}
		if ok, err := s.store.ClaimOptimize(ctx, o, info.ModTime().UnixNano()); err != nil || !ok {
			t.Fatalf("claim %v %v", ok, err)
		}
		path := filepath.Join(dir, fmt.Sprintf("%d-720-%s.mp4", f.ID, o.JobID))
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if ok, err := s.store.FinishOptimize(ctx, o, path, int64(len(content))); err != nil || !ok {
			t.Fatalf("finish %v %v", ok, err)
		}
		list, err := s.store.FileOptimized(ctx, f.ID)
		if err != nil || len(list) != 1 {
			t.Fatalf("copy %+v %v", list, err)
		}
		return list[0]
	}
	stream := func(url string, want int, content string, rangeHeader string) {
		t.Helper()
		r := request("GET", url, "")
		if rangeHeader != "" {
			r.Header.Set("Range", rangeHeader)
		}
		w := httptest.NewRecorder()
		s.stream(w, r)
		if w.Code != want || (content != "" && w.Body.String() != content) {
			t.Fatalf("stream %s: %d %s", url, w.Code, w.Body.String())
		}
	}
	caps := clientCaps{Containers: []string{"mp4"}, Video: []string{"h264"}, Audio: []string{"aac"}}
	old := makeCopy(nil, "first copy")
	oldURL := optimizedURL(item, &old)
	stream(oldURL, 200, "first copy", "")
	stream(oldURL, 206, "fir", "bytes=0-2")
	if picked := s.optimizedFor(request("POST", "/", ""), f, caps, 720); picked == nil {
		t.Fatal("valid copy not selected")
	}
	// Same size and saved seconds: the higher-resolution conversion snapshot still detects it.
	if err := os.Chtimes(source, modified, modified.Add(time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if picked := s.optimizedFor(request("POST", "/", ""), f, caps, 720); picked != nil {
		t.Fatal("unscanned timestamp change selected old copy")
	}
	stream(oldURL, 404, "", "")
	stream(fmt.Sprintf("/api/items/%s/stream?file=%d", item, f.ID), 200, "original source", "")
	if err := os.Chtimes(source, modified, modified); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(old.Path); err != nil {
		t.Fatal(err)
	}
	if picked := s.optimizedFor(request("POST", "/", ""), f, caps, 720); picked != nil {
		t.Fatal("missing artifact selected")
	}
	stream(oldURL, 404, "", "")
	w := httptest.NewRecorder()
	s.optimizeItems(w, request("POST", "/api/optimize", fmt.Sprintf(`{"item_ids":[%q],"quality":720}`, item)))
	var queued struct {
		Queued int `json:"queued"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &queued); err != nil || w.Code != 200 || queued.Queued != 1 {
		t.Fatalf("missing copy not repairable: %d %s %v", w.Code, w.Body.String(), err)
	}
	next, err := s.store.NextOptimize(ctx)
	if err != nil || next == nil || next.JobID == old.JobID {
		t.Fatalf("repair reused attempt %+v %v", next, err)
	}
	current := makeCopy(next, "second copy")
	stream(optimizedURL(item, &current), 200, "second copy", "")
	stream(oldURL, 404, "", "") // A previously issued URL cannot silently switch generations.
	stream(fmt.Sprintf("/api/items/%s/stream?file=%d&optimized=720", item, f.ID), 200, "second copy", "")
	if err := os.WriteFile(source, []byte("replacement source content"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	f.Size, f.ModTime = info.Size(), info.ModTime().Unix()
	if _, err := s.store.SaveFile(ctx, f, 2, 2); err != nil {
		t.Fatal(err)
	}
	stream(optimizedURL(item, &current), 404, "", "")
	if picked := s.optimizedFor(request("POST", "/", ""), f, caps, 720); picked != nil {
		t.Fatal("scanned replacement selected old copy")
	}
}

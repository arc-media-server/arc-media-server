package upkeep

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"ams/internal/artwork"
	"ams/internal/store"
)

func TestCleanCachesKeepsReferencesAndScope(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "cache.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(ctx, "Offline library", "movies", "tmdb_aired", []string{"/offline"})
	if err != nil {
		t.Fatal(err)
	}
	fid, err := st.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: "/offline/film.mkv", Size: 1, ModTime: 1}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.MarkMissing(ctx, lib, 2, 3); err != nil {
		t.Fatal(err)
	}
	id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Retained"}, "retained")
	if err != nil {
		t.Fatal(err)
	}
	poster := "/used.jpg"
	if err := st.EditItem(ctx, id, store.ItemEdit{Poster: &poster}); err != nil {
		t.Fatal(err)
	}
	key := artwork.Key(artwork.Source("movie", "primary", poster))
	orphan := artwork.Key("https://fixture.test/orphan.jpg")
	alive := strconv.FormatInt(fid, 10)
	hash := "0123456789abcdef01234567"
	keep := []string{"artwork/" + key[:2] + "/" + key, "trickplay/" + alive + "/info.json", "trickplay/" + alive + "/0.jpg", "subtitles/" + alive + "/" + hash + ".json", "subtitles/downloaded/" + alive + "/en.1.srt", "videothumbs/" + alive + ".jpg", "optimized/" + alive + "-720-attempt.mp4", "subtitles/" + hash + ".json", "artwork/notes.txt", "trickplay/999/private.txt", "trickplay/+999/0.jpg", "backups/ams-before-delete.db", "uploads/picture.jpg", "media/film.mkv", "certs/key.pem", "transcode/active.ts"}
	remove := []string{"artwork/" + orphan[:2] + "/" + orphan, "trickplay/999/info.json", "trickplay/999/0.jpg", "subtitles/999/" + hash + ".json", "subtitles/downloaded/999/en.1.srt", "videothumbs/999.jpg", "optimized/999-720-attempt.mp4"}
	write := func(name string, old bool) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
		if old {
			when := time.Now().Add(-2 * time.Hour)
			os.Chtimes(p, when, when)
		}
	}
	for _, p := range append(append([]string{}, keep...), remove...) {
		write(p, true)
	}
	write("videothumbs/998.jpg", false)
	keep = append(keep, "videothumbs/998.jpg")
	result, err := CleanCaches(ctx, st, dir, func() bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != len(remove) || result.Bytes != int64(4*len(remove)) || result.Legacy != 1 {
		t.Fatalf("result=%+v", result)
	}
	for _, p := range remove {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); !os.IsNotExist(err) {
			t.Errorf("orphan retained: %s %v", p, err)
		}
	}
	for _, p := range keep {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		if err != nil || string(b) != "data" {
			t.Errorf("live/protected file changed: %s %v", p, err)
		}
	}
	if _, err := CleanCaches(ctx, st, dir, func() bool { return false }); err != context.Canceled {
		t.Fatalf("busy=%v", err)
	}
}

func TestCleanCachesRefusesSymlinkRoot(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "artwork")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "cache.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := CleanCaches(ctx, st, dir, nil); err == nil {
		t.Fatal("followed a cache-root symlink")
	}
}

func TestCleanCachesDoesNotFollowNestedSymlinks(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "trickplay"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(outside, "0.jpg")
	if err := os.WriteFile(path, []byte("media"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "trickplay", "999")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "cache.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	result, err := CleanCaches(ctx, st, dir, nil)
	if err != nil || result.Files != 0 {
		t.Fatalf("followed symlink: %+v %v", result, err)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "media" {
		t.Fatalf("outside bytes changed: %q %v", b, err)
	}
}

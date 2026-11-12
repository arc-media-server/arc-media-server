package store

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPrivateFileAssetRevisionSurvivesAnalysisAndChangesWithSource(t *testing.T) {
	st := openTest(t)
	lib, err := st.SyncLibrary(t.Context(), "Fixture", "movies", "tmdb_aired", []string{"/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	f := &MediaFile{LibraryID: lib, Path: "/fixture/film.mkv", Size: 100, ModTime: 10}
	id, err := st.SaveFile(t.Context(), f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	key := f.CacheKey
	if len(key) != 32 {
		t.Fatalf("source identity missing: %q", key)
	}
	f.ID = id
	f.Bitrate = 1000
	if _, err := st.SaveAnalysis(t.Context(), f, 2); err != nil {
		t.Fatal(err)
	}
	if f.CacheKey != key {
		t.Fatal("analysis changed source identity")
	}
	if _, err := st.SaveFile(t.Context(), f, 2, 3); err != nil {
		t.Fatal(err)
	}
	if f.CacheKey != key {
		t.Fatal("unchanged scan changed source identity")
	}
	f.Size++
	if _, err := st.SaveFile(t.Context(), f, 2, 4); err != nil {
		t.Fatal(err)
	}
	if f.CacheKey == key {
		t.Fatal("new source revision inherited asset identity")
	}
	encoded, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), f.CacheKey) || strings.Contains(string(encoded), "cache_key") {
		t.Fatal("private cache identity exposed in API JSON")
	}
}

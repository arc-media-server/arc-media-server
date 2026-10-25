package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestSaveAnalysisPreservesPresenceIdentityAndRejectsStaleRevision(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Kept"}, "kept")
	if err != nil {
		t.Fatal(err)
	}
	file := MediaFile{LibraryID: lib, Path: "movie.mkv", Size: 123, ModTime: 4, DurationMS: 1000}
	file.ID, err = st.SaveFile(ctx, &file, 1, 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetFileItems(ctx, file.ID, []string{item}, 0); err != nil {
		t.Fatal(err)
	}
	file.DurationMS = 2000
	file.Streams = []Stream{{Index: 0, Kind: "video", Codec: "hevc", DolbyVision: true}}
	id, err := st.SaveAnalysis(ctx, &file, 5)
	if err != nil || id != file.ID {
		t.Fatalf("analysis save %d %v", id, err)
	}
	var seen, probedAt int64
	var missing any
	if err := st.db.QueryRowContext(ctx, `SELECT last_seen_at,probed_at,missing_since FROM media_files WHERE id=?`, id).Scan(&seen, &probedAt, &missing); err != nil {
		t.Fatal(err)
	}
	if seen != 17 || probedAt != 1 || missing != nil {
		t.Fatalf("presence changed %d %v", seen, missing)
	}
	files, err := st.ItemFiles(ctx, item)
	if err != nil || len(files) != 1 || files[0].ID != file.ID || files[0].DurationMS != 2000 || !files[0].Streams[0].DolbyVision {
		t.Fatalf("identity/analysis %+v %v", files, err)
	}
	// A concurrent saved revision change must not be overwritten by late analysis.
	if _, err := st.db.ExecContext(ctx, `UPDATE media_files SET size=size+1 WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	file.DurationMS = 3000
	if _, err := st.SaveAnalysis(ctx, &file, 6); err == nil {
		t.Fatal("stale revision accepted")
	}
	file.Size++
	if _, err := st.db.ExecContext(ctx, `UPDATE media_files SET missing_since=99 WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveAnalysis(ctx, &file, 6); err == nil {
		t.Fatal("missing file resurrected")
	}
}

func TestManualAnalysisCursorDrainsBeyondOnePage(t *testing.T) {
	st, err := Open(t.Context(), filepath.Join(t.TempDir(), "test.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 205; i++ {
		if _, err := st.SaveFile(t.Context(), &MediaFile{LibraryID: lib, Path: fmt.Sprintf("%03d.mkv", i)}, 1, 1); err != nil {
			t.Fatal(err)
		}
	}
	var after int64
	seen := map[int64]bool{}
	for {
		page, err := st.AnalysisPage(t.Context(), lib, after)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, file := range page {
			if seen[file.ID] {
				t.Fatal("duplicate page")
			}
			seen[file.ID] = true
			after = file.ID
		}
	}
	if len(seen) != 205 {
		t.Fatalf("truncated analysis scope: %d", len(seen))
	}
}

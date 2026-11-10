package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLibraryRootMoveRetainsPerFileState(t *testing.T) {
	for _, saveAll := range []bool{false, true} {
		t.Run(map[bool]string{false: "core", true: "settings"}[saveAll], func(t *testing.T) {
			st := openTest(t)
			ctx := t.Context()
			oldRoot, newRoot := t.TempDir(), t.TempDir()
			oldPath, newPath := filepath.Join(oldRoot, "Feature.mkv"), filepath.Join(newRoot, "Feature.mkv")
			at := time.Unix(1700000000, 0)
			for _, p := range []string{oldPath, newPath} {
				if err := os.WriteFile(p, []byte("fixture bytes"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(p, at, at); err != nil {
					t.Fatal(err)
				}
			}
			lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{oldRoot})
			if err != nil {
				t.Fatal(err)
			}
			f := &MediaFile{LibraryID: lib, Path: oldPath, Size: 13, ModTime: at.Unix(), Container: "matroska", DurationMS: 90000, FormatProbeVersion: 2, Streams: []Stream{{Index: 0, Kind: "video", Codec: "hevc"}}, Chapters: []Chapter{{StartMS: 0, EndMS: 40000, Title: "One"}}}
			f.ID, err = st.SaveFile(ctx, f, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			var items []string
			for _, title := range []string{"First part", "Second part"} {
				id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
				if err != nil {
					t.Fatal(err)
				}
				items = append(items, id)
			}
			if err := st.SetFileItems(ctx, f.ID, items, 2); err != nil {
				t.Fatal(err)
			}
			if err := st.SetManualMarkers(ctx, f.ID, []Marker{{Kind: "intro", StartMS: 1000, EndMS: 2000}}); err != nil {
				t.Fatal(err)
			}
			beforeMarkers, err := st.FileMarkers(ctx, f.ID)
			if err != nil {
				t.Fatal(err)
			}
			o := queueCopy(t, st, f)
			finishCopy(t, st, o, filepath.Join(t.TempDir(), "optimized.mp4"))
			if saveAll {
				before, err := st.GetLibrary(ctx, lib)
				if err != nil {
					t.Fatal(err)
				}
				after := *before
				after.Paths = []string{newRoot}
				after.Trickplay = true
				err = st.SaveLibrarySettings(ctx, *before, after, true)
			} else {
				err = st.UpdateLibrary(ctx, lib, "Movies", "tmdb_aired", []string{newRoot})
			}
			if err != nil {
				t.Fatal(err)
			}
			moved, err := st.GetFileByPath(ctx, newPath)
			if err != nil || moved == nil {
				t.Fatalf("file state lost on root move: %+v %v", moved, err)
			}
			if moved.ID != f.ID || moved.Missing || !moved.Probed || moved.Container != "matroska" || moved.DurationMS != 90000 {
				t.Errorf("file revision changed: %+v", moved)
			}
			streams, err := st.fileStreams(ctx, f.ID)
			if err != nil || len(streams) != 1 || streams[0].Codec != "hevc" {
				t.Fatalf("probe lost: %+v %v", streams, err)
			}
			markers, err := st.FileMarkers(ctx, f.ID)
			if err != nil || !reflect.DeepEqual(markers, beforeMarkers) {
				t.Fatalf("markers changed: %+v %v", markers, err)
			}
			for _, id := range items {
				files, err := st.ItemFiles(ctx, id)
				if err != nil || len(files) != 1 || files[0].ID != f.ID || files[0].Part != 2 {
					t.Errorf("split link lost: %+v %v", files, err)
				}
			}
			copies, err := st.FileOptimized(ctx, f.ID)
			if err != nil || len(copies) != len(items) {
				t.Fatalf("copy links lost: %+v %v", copies, err)
			}
			for _, copy := range copies {
				if copy.Status != "done" || copy.SourcePath != newPath || !copy.MatchesSource(moved) {
					t.Fatalf("copy provenance lost: %+v", copy)
				}
			}
		})
	}
}

func TestRemovedLibraryRootsRetainMissingFileStateWithoutGuessing(t *testing.T) {
	for _, scenario := range []string{"different revision", "nested root", "multiple roots", "already indexed target"} {
		t.Run(scenario, func(t *testing.T) {
			st := openTest(t)
			ctx := t.Context()
			oldRoot, newRoot := t.TempDir(), t.TempDir()
			paths := []string{oldRoot}
			if scenario == "multiple roots" {
				paths = append(paths, t.TempDir())
			}
			if scenario == "nested root" {
				newRoot = filepath.Join(oldRoot, "narrower")
				if err := os.Mkdir(newRoot, 0700); err != nil {
					t.Fatal(err)
				}
			}
			lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", paths)
			if err != nil {
				t.Fatal(err)
			}
			oldPath := filepath.Join(oldRoot, "Feature.mkv")
			f := &MediaFile{LibraryID: lib, Path: oldPath, Size: 13, ModTime: 1700000000, Container: "matroska"}
			f.ID, err = st.SaveFile(ctx, f, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(newRoot, "Feature.mkv")
			if err := os.WriteFile(target, []byte("different bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			if scenario == "already indexed target" {
				info, err := os.Stat(target)
				if err != nil {
					t.Fatal(err)
				}
				f.Size = info.Size()
				f.ModTime = info.ModTime().Unix()
				if _, err := st.SaveFile(ctx, f, 1, 1); err != nil {
					t.Fatal(err)
				}
				if _, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: target, Size: f.Size, ModTime: f.ModTime}, 1, 1); err != nil {
					t.Fatal(err)
				}
			}
			if err := st.UpdateLibrary(ctx, lib, "Movies", "tmdb_aired", []string{newRoot}); err != nil {
				t.Fatal(err)
			}
			retained, err := st.GetFileByPath(ctx, oldPath)
			if err != nil || retained == nil || retained.ID != f.ID || !retained.Missing {
				t.Fatalf("removed file state deleted or incorrectly mapped: %+v %v", retained, err)
			}
		})
	}
}

func TestRootRelocationRejectsStaleSnapshotAndInFlightCopies(t *testing.T) {
	for _, change := range []string{"saved revision", "queued copy", "copy nanoseconds"} {
		t.Run(change, func(t *testing.T) {
			st := openTest(t)
			ctx := t.Context()
			oldRoot, newRoot := t.TempDir(), t.TempDir()
			at := time.Unix(1700000000, 0)
			newPath := filepath.Join(newRoot, "Feature.mkv")
			if err := os.WriteFile(newPath, []byte("fixture bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(newPath, at, at); err != nil {
				t.Fatal(err)
			}
			lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{oldRoot})
			if err != nil {
				t.Fatal(err)
			}
			item, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			f := &MediaFile{LibraryID: lib, Path: filepath.Join(oldRoot, "Feature.mkv"), Size: 13, ModTime: at.Unix()}
			f.ID, err = st.SaveFile(ctx, f, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.SetFileItems(ctx, f.ID, []string{item}, 0); err != nil {
				t.Fatal(err)
			}
			o := queueCopy(t, st, f)
			if change == "copy nanoseconds" {
				finishCopy(t, st, o, "fixture-copy.mp4")
				if _, err := st.db.ExecContext(ctx, `UPDATE optimized_versions SET source_mtime_ns=source_mtime_ns+1`); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := st.prepareLibraryRelocation(ctx, lib, []string{newRoot})
			if err != nil {
				t.Fatal(err)
			}
			if change == "saved revision" {
				if _, err := st.db.ExecContext(ctx, `UPDATE media_files SET size=size+1 WHERE id=?`, f.ID); err != nil {
					t.Fatal(err)
				}
			}
			tx, err := st.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if err := updateLibraryTx(ctx, tx, lib, "Movies", "tmdb_aired", []string{newRoot}, nil, plan); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			path := newPath
			if change == "saved revision" {
				path = f.Path
			}
			saved, err := st.GetFileByPath(ctx, path)
			if err != nil || saved == nil || saved.ID != f.ID {
				t.Fatalf("saved identity lost: %+v %v", saved, err)
			}
			if change == "saved revision" {
				if !saved.Missing {
					t.Fatal("stale relocation was applied")
				}
				return
			}
			var status, jobID string
			if err := st.db.QueryRowContext(ctx, "SELECT status, job_id FROM optimized_versions WHERE file_id=?", f.ID).Scan(&status, &jobID); err != nil {
				t.Fatal(err)
			}
			if status != "failed" || jobID == o.JobID {
				t.Fatalf("old job retained: status=%s sameID=%v", status, jobID == o.JobID)
			}
			if ok, err := st.FinishOptimize(ctx, o, "stale-copy.mp4", 4); err != nil || ok {
				t.Fatalf("stale completion accepted: %v %v", ok, err)
			}
		})
	}
}

package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func optimizedFixture(t *testing.T) (*Store, *MediaFile, string) {
	t.Helper()
	ctx := t.Context()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Example"}, "example")
	if err != nil {
		t.Fatal(err)
	}
	f := &MediaFile{LibraryID: lib, Path: "/synthetic/movie.mkv", Size: 10, ModTime: 1}
	f.ID, err = st.SaveFile(ctx, f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetFileItems(ctx, f.ID, []string{id}, 0); err != nil {
		t.Fatal(err)
	}
	return st, f, id
}

func queueCopy(t *testing.T, st *Store, f *MediaFile) *Optimized {
	t.Helper()
	n, err := st.QueueOptimize(t.Context(), []int64{f.ID}, 720)
	if err != nil || n != 1 {
		t.Fatalf("queue %d: %v", n, err)
	}
	o, err := st.NextOptimize(t.Context())
	if err != nil || o == nil || !o.MatchesSource(f) {
		t.Fatalf("queued source %+v: %v", o, err)
	}
	return o
}

func finishCopy(t *testing.T, st *Store, o *Optimized, path string) {
	t.Helper()
	ok, err := st.ClaimOptimize(t.Context(), o, o.SourceMTime*1_000_000_000)
	if err != nil || !ok {
		t.Fatalf("claim %v: %v", ok, err)
	}
	ok, err = st.FinishOptimize(t.Context(), o, path, 4)
	if err != nil || !ok {
		t.Fatalf("finish %v: %v", ok, err)
	}
}

func TestOptimizedSourceReplacement(t *testing.T) {
	for _, change := range []string{"size", "mtime"} {
		t.Run(change, func(t *testing.T) {
			ctx := t.Context()
			st, f, item := optimizedFixture(t)
			o := queueCopy(t, st, f)
			finishCopy(t, st, o, "/synthetic/old.mp4")
			if _, err := st.SaveFile(ctx, f, 2, 2); err != nil {
				t.Fatal(err)
			}
			if n, err := st.QueueOptimize(ctx, []int64{f.ID}, 720); err != nil || n != 0 {
				t.Fatalf("unchanged requeued %d: %v", n, err)
			}
			if copies, err := st.FileOptimized(ctx, f.ID); err != nil || len(copies) != 1 {
				t.Fatalf("unchanged copy lost: %+v %v", copies, err)
			}
			u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.ReportProgress(ctx, u.ID, item, 90_000, 600_000); err != nil {
				t.Fatal(err)
			}
			title := "My title"
			if err := st.EditItem(ctx, item, ItemEdit{Title: &title}); err != nil {
				t.Fatal(err)
			}
			if change == "size" {
				f.Size++
			} else {
				f.ModTime++
			}
			id, err := st.SaveFile(ctx, f, 3, 3)
			if err != nil || id != f.ID {
				t.Fatalf("identity changed %d: %v", id, err)
			}
			if copies, err := st.FileOptimized(ctx, f.ID); err != nil || len(copies) != 0 {
				t.Fatalf("changed source still playable: %+v %v", copies, err)
			}
			list, err := st.OptimizedVersions(ctx)
			if err != nil || len(list) != 1 || list[0].Status != "failed" || list[0].Path != "/synthetic/old.mp4" {
				t.Fatalf("invalidated copy not removable: %+v %v", list, err)
			}
			next := queueCopy(t, st, f)
			if next.JobID == o.JobID {
				t.Fatal("replacement reused the old attempt")
			}
			finishCopy(t, st, next, "/synthetic/new.mp4")
			it, err := st.GetItem(ctx, item)
			if err != nil || it.Title != title {
				t.Fatalf("edit lost: %+v %v", it, err)
			}
			data, err := st.UserDataFor(ctx, u.ID, []string{item})
			if err != nil || data[item].PositionMS != 90_000 {
				t.Fatalf("resume lost: %+v %v", data, err)
			}
		})
	}
}

func TestOptimizedSupersededCompletion(t *testing.T) {
	for _, action := range []string{"changed", "deleted", "restart"} {
		t.Run(action, func(t *testing.T) {
			ctx := t.Context()
			st, f, _ := optimizedFixture(t)
			old := queueCopy(t, st, f)
			if ok, err := st.ClaimOptimize(ctx, old, 1_000_000_000); err != nil || !ok {
				t.Fatalf("claim %v %v", ok, err)
			}
			var next *Optimized
			switch action {
			case "changed":
				f.Size++
				if _, err := st.SaveFile(ctx, f, 2, 2); err != nil {
					t.Fatal(err)
				}
			case "deleted":
				if _, err := st.DeleteOptimized(ctx, f.ID, 720); err != nil {
					t.Fatal(err)
				}
			case "restart":
				if err := st.ResetConverting(ctx); err != nil {
					t.Fatal(err)
				}
				next, _ = st.NextOptimize(ctx)
			}
			if next == nil {
				next = queueCopy(t, st, f)
			}
			finishCopy(t, st, next, "/synthetic/new.mp4")
			if ok, err := st.FinishOptimize(ctx, old, "/synthetic/stale.mp4", 9); err != nil || ok {
				t.Fatalf("old completion accepted %v %v", ok, err)
			}
			if ok, err := st.FailOptimize(ctx, old, "late failure"); err != nil || ok {
				t.Fatalf("old failure accepted %v %v", ok, err)
			}
			copies, err := st.FileOptimized(ctx, f.ID)
			if err != nil || len(copies) != 1 || copies[0].Path != "/synthetic/new.mp4" || copies[0].JobID != next.JobID {
				t.Fatalf("new result overwritten: %+v %v", copies, err)
			}
		})
	}
}

func TestOptimizedRejectsChangedQueuedSource(t *testing.T) {
	st, f, _ := optimizedFixture(t)
	o := queueCopy(t, st, f)
	f.ModTime++
	if _, err := st.SaveFile(t.Context(), f, 2, 2); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.ClaimOptimize(t.Context(), o, 1); err != nil || ok {
		t.Fatalf("stale queue claimed: %v %v", ok, err)
	}
}

func TestOptimizedUpgradeAndPersistence(t *testing.T) {
	ctx := context.Background()
	old := oldBackupThrough(t, 48)
	db, err := sql.Open("sqlite", old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO media_files(id,library_id,path,size,mtime,probed_at,last_seen_at) VALUES(91,1,'legacy.mkv',10,2,1,1);
	 INSERT INTO item_files(item_id,file_id,part) VALUES('movie',91,0);
	 INSERT INTO optimized_versions(file_id,quality,path,size,status,created_at) VALUES(91,720,'legacy-copy.mp4',4,'done',1),(91,1080,'',0,'converting',1)`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "before-upgrade.db")
	st, err := Open(ctx, old, Options{BeforeMigrate: func(s *Store) error { return s.BackupTo(ctx, backup) }})
	if err != nil {
		t.Fatal(err)
	}
	list, err := st.OptimizedVersions(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("legacy copies %+v %v", list, err)
	}
	for _, o := range list {
		if o.JobID == "" || o.SourcePath != "legacy.mkv" {
			t.Fatalf("missing source snapshot: %+v", o)
		}
		if o.Quality == 720 && (o.Status != "failed" || o.Path != "legacy-copy.mp4") {
			t.Fatalf("unknown old copy trusted or lost: %+v", o)
		}
		if o.Quality == 1080 && o.Status != "queued" {
			t.Fatalf("interrupted legacy work not queued: %+v", o)
		}
	}
	if copies, err := st.FileOptimized(ctx, 91); err != nil || len(copies) != 0 {
		t.Fatalf("legacy done reused: %+v %v", copies, err)
	}
	o, err := st.NextOptimize(ctx)
	if err != nil || o == nil {
		t.Fatalf("no resumed job: %v", err)
	}
	finishCopy(t, st, o, "new-copy.mp4")
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = Open(ctx, old, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	copies, err := st.FileOptimized(ctx, 91)
	if err != nil || len(copies) != 1 || copies[0].JobID != o.JobID {
		t.Fatalf("revision lost on reopen: %+v %v", copies, err)
	}
	if err := CheckBackup(ctx, backup); err != nil {
		t.Fatalf("pre-upgrade backup invalid: %v", err)
	}
}

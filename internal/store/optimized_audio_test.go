package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOptimizedAudioUpgradeAndPersistence(t *testing.T) {
	ctx := t.Context()
	old := oldBackupThrough(t, 49)
	db, err := sql.Open("sqlite", old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO media_files(id,library_id,path,size,mtime,probed_at,last_seen_at) VALUES(91,1,'source.mkv',10,2,1,1);
 INSERT INTO item_files(item_id,file_id,part) VALUES('movie',91,0);
 INSERT INTO optimized_versions(file_id,quality,path,size,status,created_at,source_path,source_size,source_mtime,source_mtime_ns,job_id)
 VALUES(91,720,'older-copy.mp4',4,'done',1,'source.mkv',10,2,2000000000,'old-job');`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "before-audio-upgrade.db")
	st, err := Open(ctx, old, Options{BeforeMigrate: func(s *Store) error { return s.BackupTo(ctx, backup) }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	list, err := st.OptimizedVersions(ctx)
	if err != nil || len(list) != 1 || list[0].Status != "failed" || list[0].AudioStream != -2 || list[0].Path != "older-copy.mp4" {
		t.Fatalf("legacy provenance inferred/artifact lost: %+v %v", list, err)
	}
	if list[0].MatchesAudio(nil) || list[0].MatchesAudio(&Stream{Index: 1}) {
		t.Fatal("unknown audio accepted")
	}
	if _, err := st.QueueOptimize(ctx, []int64{91}, 720); err != nil {
		t.Fatal(err)
	}
	o, err := st.NextOptimize(ctx)
	if err != nil || o == nil || o.AudioStream != -2 {
		t.Fatalf("queue: %+v %v", o, err)
	}
	o.AudioStream, o.AudioLanguage, o.AudioTitle = 2, "fra", "French main"
	finishCopy(t, st, o, "new-copy.mp4")
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = Open(ctx, old, Options{})
	if err != nil {
		t.Fatal(err)
	}
	copies, err := st.FileOptimized(ctx, 91)
	if err != nil || len(copies) != 1 || copies[0].AudioStream != 2 || copies[0].AudioLanguage != "fra" || copies[0].AudioTitle != "French main" {
		t.Fatalf("audio lost after restart: %+v %v", copies, err)
	}
	if !copies[0].MatchesAudio(&Stream{Index: 2}) || copies[0].MatchesAudio(&Stream{Index: 3, Language: "fra"}) || copies[0].MatchesAudio(nil) {
		t.Fatal("exact track comparison failed")
	}
	silent := Optimized{AudioStream: -1}
	if !silent.MatchesAudio(nil) || silent.MatchesAudio(&Stream{Index: 1}) {
		t.Fatal("silent copy accepted audible choice")
	}
	if err := CheckBackup(ctx, backup); err != nil {
		t.Fatal(err)
	}
}

func TestOptimizedBitrateUpgradeAndPersistence(t *testing.T) {
	ctx := t.Context()
	old := oldBackupThrough(t, 50)
	db, err := sql.Open("sqlite", old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO media_files(id,library_id,path,size,mtime,probed_at,last_seen_at) VALUES(91,1,'source.mkv',10,2,1,1);
 INSERT INTO item_files(item_id,file_id,part) VALUES('movie',91,0);
 INSERT INTO optimized_versions(file_id,quality,path,size,status,created_at,source_path,source_size,source_mtime,job_id,audio_stream,audio_language)
 VALUES(91,720,'older-copy.mp4',4,'done',1,'source.mkv',10,2,'old-job',1,'eng');`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "before-bitrate-upgrade.db")
	st, err := Open(ctx, old, Options{BeforeMigrate: func(s *Store) error { return s.BackupTo(ctx, backup) }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	list, err := st.OptimizedVersions(ctx)
	if err != nil || len(list) != 1 || list[0].Status != "done" || list[0].Bitrate != 0 || list[0].DurationMS != 0 || list[0].Path != "older-copy.mp4" {
		t.Fatalf("legacy copy altered: %+v %v", list, err)
	}
	if !list[0].WithinBitrate(0) || list[0].WithinBitrate(2000) {
		t.Fatal("unknown legacy budget policy wrong")
	}
	if _, err := st.FailOptimize(ctx, &list[0], "fixture regenerate"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.QueueOptimize(ctx, []int64{91}, 720); err != nil {
		t.Fatal(err)
	}
	o, err := st.NextOptimize(ctx)
	if err != nil || o == nil {
		t.Fatalf("next: %+v %v", o, err)
	}
	o.AudioStream, o.AudioLanguage, o.DurationMS, o.Bitrate = 1, "eng", 6000, 2000000
	finishCopy(t, st, o, "measured.mp4")
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = Open(ctx, old, Options{})
	if err != nil {
		t.Fatal(err)
	}
	list, err = st.FileOptimized(ctx, 91)
	if err != nil || len(list) != 1 || list[0].DurationMS != 6000 || list[0].Bitrate != 2000000 || !list[0].WithinBitrate(2000) || list[0].WithinBitrate(1999) {
		t.Fatalf("metadata lost: %+v %v", list, err)
	}
	if _, err := st.FailOptimize(ctx, &list[0], "fixture regenerate"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.QueueOptimize(ctx, []int64{91}, 720); err != nil {
		t.Fatal(err)
	}
	next, err := st.NextOptimize(ctx)
	if err != nil || next == nil || next.Bitrate != 0 || next.DurationMS != 0 {
		t.Fatalf("requeue retained old measurements: %+v %v", next, err)
	}
	if err := CheckBackup(ctx, backup); err != nil {
		t.Fatal(err)
	}
}

package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Every persistent table is populated, so an accidentally omitted table/column cannot pass
// merely because the fixture's optional features were empty. All values/paths/keys are fake.
func richPortableSQLite(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "source.db")
	st, err := Open(t.Context(), path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	sql := `
 INSERT INTO libraries(id,name,kind,created_at) VALUES(101,'Shows','shows',1);
 INSERT INTO library_paths VALUES(101,'/fixture-only/media');
 INSERT INTO users(id,name,password_hash,pin_hash,is_admin,created_at,library_ids,exclude_labels) VALUES('owner','Owner','fixture-password-hash','fixture-pin-hash',1,1,'101','["Blocked"]');
 INSERT INTO sessions(token_hash,user_id,via,user_agent,created_at,last_seen_at) VALUES('fixture-session','owner','password','fixture',1,1);
 INSERT INTO items(id,library_id,kind,match_key,title,sort_title,added_at,updated_at,locked_fields,poster_path) VALUES('show',101,'series','show','Edited 示例','edited',1,1,'title,poster','upload:fixture.jpg');
 INSERT INTO items(id,library_id,kind,parent_id,match_key,title,sort_title,added_at,updated_at,rating) VALUES('episode',101,'episode','show','ep:show:tvdb:42','Episode','episode',1,1,8.7);
 INSERT INTO items(id,library_id,kind,parent_id,match_key,title,sort_title,added_at,updated_at) VALUES('special',101,'episode','show','ep:show:tvdb:43','Special','special',1,1);
 INSERT INTO media_files(id,library_id,path,size,mtime,last_seen_at,probed_at,duration_ms,missing_since) VALUES(201,101,'/fixture-only/示例.mkv',9007199254740993,2,3,4,1800000,123);
 INSERT INTO media_streams(file_id,idx,kind,codec,dolby_vision,dolby_vision_profile,hdr10_plus,bit_depth,frame_rate) VALUES(201,0,'video','hevc',1,8,1,10,23.976);
 INSERT INTO media_streams(file_id,idx,kind,codec,channels,profile) VALUES(201,1,'audio','truehd',8,'Dolby TrueHD + Dolby Atmos');
 INSERT INTO media_chapters(file_id,idx,start_ms,end_ms,title) VALUES(201,0,0,9000,'Chapter');
 INSERT INTO item_files VALUES('episode',201,0);
 INSERT INTO item_genres VALUES('show','Drama');
 INSERT INTO item_genres VALUES('show','action');
 INSERT INTO item_genres VALUES('show','Zebra');
 INSERT INTO item_labels VALUES('show','Allowed');
 INSERT INTO item_labels VALUES('show','zebra');
 INSERT INTO people(id,tmdb_id,name) VALUES(401,501,'Actor');
 INSERT INTO item_people(item_id,person_id,role,character,ord,episode_count) VALUES('show',401,'actor','Fixture',0,12);
 INSERT INTO item_alt_titles VALUES('show','Alternate 示例');
 INSERT INTO item_keywords VALUES('show','keyword');
 INSERT INTO item_recommendations VALUES('show',0,1234);
 INSERT INTO item_aliases VALUES(101,'old-folder','show');
 INSERT INTO item_arrivals VALUES('show',321,1);
 INSERT INTO item_arrivals VALUES('episode',322,2);
 UPDATE arrival_clock SET sequence=2;
 INSERT INTO user_item_data(user_id,item_id,position_ms,played,play_count,last_played_at,user_rating,watchlisted_at) VALUES('owner','episode',123456,0,4,789,8,654);
 INSERT INTO continue_dismissed VALUES('owner','show',777);
 INSERT INTO play_history(id,user_id,item_id,kind,library_id,series_id,title,started_at,ended_at,watched_ms,position_ms,duration_ms) VALUES(601,'owner','episode','episode',101,'show','Episode',100,200,123456,123456,1800000);
 INSERT INTO collections(id,library_id,kind,title,sort_title,created_at,locked_fields) VALUES('collection',101,'manual','Collection','collection',1,'title');
 INSERT INTO collection_items VALUES('collection','show',7);
 INSERT INTO playlists(id,user_id,title,created_at,updated_at) VALUES('playlist','owner','Playlist',1,2);
 INSERT INTO playlist_entries(id,playlist_id,item_id,position) VALUES(701,'playlist','episode',5);
 INSERT INTO orderings(id,series_id,source,name,updated_at) VALUES(301,'show','custom','Custom',1);
 INSERT INTO ordering_entries(ordering_id,episode_id,season_number,episode_number,locked) VALUES(301,'episode',3,16,1);
 INSERT INTO ordering_seasons(ordering_id,season_number,name,overview,locked_fields) VALUES(301,3,'Edited season','Summary','name');
 INSERT INTO series_display VALUES('show','owner',301);
 INSERT INTO series_specials VALUES('show','owner',1);
 INSERT INTO episode_specials VALUES('special','episode');
 INSERT INTO season_art VALUES('show',3,'/provider.jpg','/fixture-only/season.jpg','upload:season.jpg');
 INSERT INTO media_markers VALUES(201,'intro',1000,2000,'manual',1);
 INSERT INTO background_jobs(file_id,kind,job_key,revision,state,error,updated_at) VALUES(201,'analysis','file:201','revision','queued','',0);
 INSERT INTO optimized_versions(file_id,quality,path,status,created_at,source_path,source_size,source_mtime,job_id) VALUES(201,720,'/fixture-only/optimized.mp4','done',1,'/fixture-only/示例.mkv',9007199254740993,2,'fixture-job');
 INSERT INTO extras(id,item_id,file_id,kind,title) VALUES('extra','show',201,'trailer','Trailer');
 INSERT INTO library_recommended VALUES(101,'{"rows":["recent"]}');
 INSERT INTO play_queues(id,user_id,session_hash,current_entry_id,expires_at) VALUES('queue','owner','fixture-session','entry',9999999999);
 INSERT INTO play_queue_entries VALUES('entry','queue','episode',0,0);
 INSERT INTO webhooks(id,name,url,created_at) VALUES('webhook','Fixture','https://example.invalid/fixture',1);
 INSERT INTO settings(key,value) VALUES('tmdb_api_key','fixture-key-not-real');
 `
	if _, err = st.db.ExecContext(t.Context(), sql); err != nil {
		st.Close()
		t.Fatal(err)
	}
	for _, table := range tableNames() {
		var count int
		if err = st.db.QueryRowContext(t.Context(), "SELECT count(*) FROM "+quoted(table)).Scan(&count); err != nil || count == 0 {
			t.Fatalf("fixture leaves %s empty: %v", table, err)
		}
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	return path, root
}
func TestPostgresPortableTransferEveryTableAndRoundTrip(t *testing.T) {
	target := postgresTarget(t)
	source, root := richPortableSQLite(t)
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(root, "safety.db")
	if err = SnapshotSQLite(t.Context(), source, snapshot); err != nil {
		t.Fatal(err)
	}
	report, err := TransferSQLiteWithID(t.Context(), snapshot, target, "fixture-transfer")
	if err != nil {
		t.Fatal(err)
	}
	if !report.Verified || len(report.Tables) != len(pgColumns) {
		t.Fatalf("incomplete transfer: %+v", report)
	}
	after, _ := os.ReadFile(source)
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("original SQLite was modified")
	}
	if _, ok, err := CompletedTransfer(t.Context(), snapshot, target, "fixture-transfer"); err != nil || !ok {
		t.Fatalf("recovery verification: %v %v", ok, err)
	}
	pg, err := OpenPostgres(t.Context(), target, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer pg.Close()
	owner, err := pg.GetUser(t.Context(), "owner")
	if err != nil || owner.PasswordHash != "fixture-password-hash" || owner.PINHash != "fixture-pin-hash" || len(owner.LibraryIDs) != 1 || owner.LibraryIDs[0] != 101 {
		t.Fatalf("profile/access lost: %+v %v", owner, err)
	}
	state, err := pg.UserDataFor(t.Context(), "owner", []string{"episode"})
	if err != nil || state["episode"].PositionMS != 123456 || !state["episode"].Watchlisted {
		t.Fatalf("watch state lost: %+v %v", state, err)
	}
	file, err := pg.GetFileByPath(t.Context(), "/fixture-only/示例.mkv")
	if err != nil || file.Size != 9007199254740993 || !file.Probed || !file.Missing {
		t.Fatalf("file identity/probe lost: %+v %v", file, err)
	}
	backup := filepath.Join(root, "postgres-backup.db")
	if err = pg.BackupTo(t.Context(), backup); err != nil {
		t.Fatal(err)
	}
	if err = pg.SetSetting(t.Context(), "tmdb_api_key", "changed"); err != nil {
		t.Fatal(err)
	}
	if err = pg.RestoreFrom(t.Context(), backup); err != nil {
		t.Fatal(err)
	}
	key, err := pg.Setting(t.Context(), "tmdb_api_key")
	if err != nil || key != "fixture-key-not-real" {
		t.Fatal("restore lost settings", err)
	}
	next, err := pg.SyncLibrary(t.Context(), "Next", "movies", "tmdb_aired", nil)
	if err != nil || next <= 101 {
		t.Fatalf("identity sequence not advanced: %d %v", next, err)
	}
	sqlite, err := Open(t.Context(), backup, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer sqlite.Close()
	for _, want := range report.Tables {

		got, err := tableDigest(t.Context(), sqlite.db, want.Table, false)
		if err != nil || got != want {
			t.Fatalf("portable backup mismatch %s: %v", want.Table, err)
		}
	}
}
func TestPostgresTransferCancellationAndNonemptyTarget(t *testing.T) {
	source, root := richPortableSQLite(t)
	snapshot := filepath.Join(root, "safety.db")
	if err := SnapshotSQLite(t.Context(), source, snapshot); err != nil {
		t.Fatal(err)
	}
	target := postgresTarget(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := TransferSQLite(ctx, snapshot, target); err == nil {
		t.Fatal("cancelled transfer succeeded")
	}
	check, err := InspectPostgres(t.Context(), target)
	if err != nil || !check.Empty {
		t.Fatalf("cancelled target changed: %+v %v", check, err)
	}
	pg, err := OpenPostgres(t.Context(), target, Options{})
	if err != nil {
		t.Fatal(err)
	}
	pg.SetSetting(t.Context(), "retained", "keep")
	pg.Close()
	if _, err = TransferSQLite(t.Context(), snapshot, target); err == nil {
		t.Fatal("overwrote a nonempty target")
	}
	pg, err = OpenPostgres(t.Context(), target, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer pg.Close()
	v, err := pg.Setting(t.Context(), "retained")
	if err != nil || v != "keep" {
		t.Fatal("target data changed")
	}
}

func TestPostgresRolledBackRestoreCannotRewindSequences(t *testing.T) {
	source, _ := richPortableSQLite(t)
	pg := postgresFixture(t)
	if _, err := pg.db.ExecContext(t.Context(), "SELECT setval(pg_get_serial_sequence('libraries','id'),1000,true)"); err != nil {
		t.Fatal(err)
	}
	before, err := pg.SyncLibrary(t.Context(), "Retained", "movies", "tmdb_aired", nil)
	if err != nil || before != 1001 {
		t.Fatal(before, err)
	}
	sqlite, err := sql.Open("sqlite", "file:"+source+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlite.Close()
	read, err := sqlite.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer read.Rollback()
	tx, err := pg.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = clearPortable(t.Context(), tx, true); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if _, err = copyPortable(t.Context(), read, tx, false, true); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	next, err := pg.SyncLibrary(t.Context(), "After rollback", "movies", "tmdb_aired", nil)
	if err != nil || next <= before {
		t.Fatalf("rollback rewound a nontransactional sequence: before=%d next=%d %v", before, next, err)
	}
	retained, err := pg.GetLibrary(t.Context(), before)
	if err != nil || retained == nil || retained.Name != "Retained" {
		t.Fatal("rollback lost existing data", err)
	}
}
func TestSnapshotSQLiteReadsWALWithoutChangingSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "live.db")
	st, err := Open(t.Context(), path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err = st.SetSetting(t.Context(), "wal-only", "retained"); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(dir, "snapshot.db")
	if err = SnapshotSQLite(t.Context(), path, snapshot); err != nil {
		t.Fatal(err)
	}
	copy, err := Open(t.Context(), snapshot, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	v, err := copy.Setting(t.Context(), "wal-only")
	if err != nil || v != "retained" {
		t.Fatal("WAL changes lost", err)
	}
}
func TestPostgresGeneratedBaselineTracksEverySQLiteMigration(t *testing.T) {
	st := openSQLiteTest(t)
	want, cols, err := postgresBaseline(t.Context(), st)
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := pgSchemaFS.ReadFile("postgres/baseline.sql")
	if string(actual) != want || !reflect.DeepEqual(cols, pgColumns) {
		t.Fatal("PostgreSQL baseline stale; regenerate and supply a tested PostgreSQL upgrade path")
	}
}

func TestPostgresRestoreOlderBackupUpgradesOnlyDisposableCopy(t *testing.T) {
	pg := postgresFixture(t)
	old := oldBackupThrough(t, 8)
	raw, err := sql.Open("sqlite", old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = raw.ExecContext(t.Context(), "UPDATE items SET content_rating='15' WHERE id='movie'"); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	before, err := os.ReadFile(old)
	if err != nil {
		t.Fatal(err)
	}
	if err = pg.RestoreFrom(t.Context(), old); err != nil {
		t.Fatal(err)
	}
	movie, err := pg.GetItem(t.Context(), "movie")
	if err != nil || movie == nil || movie.Title != "My edited title" {
		t.Fatalf("older backup lost edits or rating protection: %+v %v", movie, err)
	}
	var age int
	if err = pg.db.QueryRowContext(t.Context(), "SELECT min_age FROM items WHERE id='movie'").Scan(&age); err != nil || age != 15 {
		t.Fatal("restored parental age not rebuilt", age, err)
	}
	after, _ := os.ReadFile(old)
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("original older backup was modified")
	}
}

type cancelDuringTableCopy struct {
	querySource
	cancel    context.CancelFunc
	triggered bool
}

func (s *cancelDuringTableCopy) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	rows, err := s.querySource.QueryContext(ctx, q, args...)
	if err == nil && strings.Contains(q, `FROM "media_streams"`) {
		s.triggered = true
		s.cancel()
	}
	return rows, err
}
func TestPostgresCancellationDuringCopyPreservesExistingData(t *testing.T) {
	pg := postgresFixture(t)
	if err := pg.SetSetting(t.Context(), "retained", "original"); err != nil {
		t.Fatal(err)
	}
	source, _ := richPortableSQLite(t)
	sqlite, err := sql.Open("sqlite", "file:"+source+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlite.Close()
	read, err := sqlite.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer read.Rollback()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	wrapped := &cancelDuringTableCopy{querySource: read, cancel: cancel}
	tx, err := pg.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = clearPortable(ctx, tx, true); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	_, err = copyPortable(ctx, wrapped, tx, false, true)
	tx.Rollback()
	if err == nil || !wrapped.triggered {
		t.Fatal("copy was not cancelled midway", err)
	}
	if err = pg.SetSetting(t.Context(), "after-cancel", "still writable"); err != nil {
		t.Fatal(err)
	}
	v, err := pg.Setting(t.Context(), "retained")
	if err != nil || v != "original" {
		t.Fatal("cancelled copy published partial state", err)
	}
	var items int
	if err = pg.db.QueryRowContext(t.Context(), "SELECT count(*) FROM items").Scan(&items); err != nil || items != 0 {
		t.Fatal("cancelled copy left imported items", items, err)
	}
}

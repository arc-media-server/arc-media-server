package store

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func downloadStoreFixture(t *testing.T) (*Store, string, string, string, MediaFile) {
	t.Helper()
	st, user, session, ids := queueStoreFixture(t)
	ctx := t.Context()
	yes := true
	if err := st.UpdateUser(ctx, user, UserUpdate{AllowDownloads: &yes}); err != nil {
		t.Fatal(err)
	}
	it, err := st.GetItem(ctx, ids[0])
	if err != nil {
		t.Fatal(err)
	}
	f := MediaFile{LibraryID: it.LibraryID, Path: filepath.Join(t.TempDir(), "fixture.mp4"), Size: 123, ModTime: 20}
	f.ID, err = st.SaveFile(ctx, &f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.SetFileItems(ctx, f.ID, []string{it.ID}, 0); err != nil {
		t.Fatal(err)
	}
	return st, user, session, it.ID, f
}
func TestDownloadStoreIdempotencyOwnershipLimitAndRevocation(t *testing.T) {
	st, user, session, item, f := downloadStoreFixture(t)
	ctx := t.Context()
	d, created, err := st.CreateDownload(ctx, user, session, item, "request", f)
	if err != nil || !created {
		t.Fatalf("create %v %v", created, err)
	}
	repeat, created, err := st.CreateDownload(ctx, user, session, item, "request", f)
	if err != nil || created || repeat.ID != d.ID {
		t.Fatal("retry did not reuse operation")
	}
	changed := f
	changed.ID++
	if _, _, err = st.CreateDownload(ctx, user, session, item, "request", changed); !errors.Is(err, ErrDownloadConflict) {
		t.Fatalf("changed payload: %v", err)
	}
	if _, err = st.Download(ctx, user, "another-device", d.ID); !errors.Is(err, ErrDownloadNotFound) {
		t.Fatalf("cross device %v", err)
	}
	if _, err = st.DownloadSource(ctx, d); err != nil {
		t.Fatal(err)
	}
	no := false
	if err = st.UpdateUser(ctx, user, UserUpdate{AllowDownloads: &no}); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DownloadSource(ctx, d); !errors.Is(err, ErrDownloadNotFound) {
		t.Fatalf("grant revocation %v", err)
	}
	yes := true
	_ = st.UpdateUser(ctx, user, UserUpdate{AllowDownloads: &yes})
	if err = st.SetSessionUser(ctx, session, ""); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DownloadSource(ctx, d); !errors.Is(err, ErrDownloadNotFound) {
		t.Fatalf("profile switch %v", err)
	}
	if err = st.SetSessionUser(ctx, session, user); err != nil {
		t.Fatal(err)
	}
	if err = st.DeleteDownload(ctx, user, session, d.ID); err != nil {
		t.Fatal(err)
	}
	if published, err := st.FinishDownload(ctx, d, "old digest", 1, ""); err != nil || published {
		t.Fatal("deleted attempt published")
	}
	for i := 0; i < 100; i++ {
		if _, _, err = st.CreateDownload(ctx, user, session, item, fmt.Sprint(i), f); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err = st.CreateDownload(ctx, user, session, item, "overflow", f); !errors.Is(err, ErrDownloadLimit) {
		t.Fatalf("limit %v", err)
	}
	if _, created, err = st.CreateDownload(ctx, user, session, item, "0", f); err != nil || created {
		t.Fatal("retry rejected at limit")
	}
	if err = st.DeleteSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	list, err := st.DownloadRecords(ctx, user, session)
	if err != nil || len(list) != 0 {
		t.Fatal("revoked session retained download records")
	}
}
func TestDownloadConcurrentAdmissionIsIdempotent(t *testing.T) {
	st, user, session, item, f := downloadStoreFixture(t)
	ctx := t.Context()
	var wg sync.WaitGroup
	ids := make(chan string, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, _, err := st.CreateDownload(ctx, user, session, item, "same", f)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- d.ID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for got := range ids {
		if id != "" && id != got {
			t.Fatal("duplicate records")
		}
		id = got
	}
	list, err := st.DownloadRecords(ctx, user, session)
	if err != nil || len(list) != 1 {
		t.Fatalf("records %d %v", len(list), err)
	}
}

func TestDownloadRecordsAndGrantSurviveBackupRestore(t *testing.T) {
	st, user, session, item, f := downloadStoreFixture(t)
	ctx := t.Context()
	d, _, err := st.CreateDownload(ctx, user, session, item, "backup", f)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup.db")
	if err = st.BackupTo(ctx, path); err != nil {
		t.Fatal(err)
	}
	if err = st.DeleteDownload(ctx, user, session, d.ID); err != nil {
		t.Fatal(err)
	}
	no := false
	if err = st.UpdateUser(ctx, user, UserUpdate{AllowDownloads: &no}); err != nil {
		t.Fatal(err)
	}
	if err = st.RestoreFrom(ctx, path); err != nil {
		t.Fatal(err)
	}
	got, err := st.Download(ctx, user, session, d.ID)
	if err != nil || got.Revision != d.Revision || got.State != "preparing" {
		t.Fatal("descriptor not restored")
	}
	u, err := st.GetUser(ctx, user)
	if err != nil || !u.AllowDownloads {
		t.Fatal("grant not restored")
	}
}

func TestDownloadConcurrentAdmissionCannotExceedDeviceLimit(t *testing.T) {
	st, user, session, item, f := downloadStoreFixture(t)
	ctx := t.Context()
	for i := 0; i < 99; i++ {
		if _, _, err := st.CreateDownload(ctx, user, session, item, fmt.Sprint(i), f); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	admitted := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := st.CreateDownload(ctx, user, session, item, fmt.Sprintf("concurrent-%d", i), f)
			if err != nil && !errors.Is(err, ErrDownloadLimit) {
				t.Error(err)
			}
			admitted <- err == nil
		}(i)
	}
	wg.Wait()
	close(admitted)
	n := 0
	for yes := range admitted {
		if yes {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("admitted %d at99/100", n)
	}
}

func TestDownloadMigrationPreservesExistingSQLiteStateAndSafetyBackup(t *testing.T) {
	st, user, session, item, f := downloadStoreFixture(t)
	ctx := t.Context()
	// Reconstruct the preceding SQLite schema in a disposable fixture, keeping real profile,
	// session, library/item/file rows rather than testing only an empty installation.
	for _, query := range []string{`DROP TABLE downloads`, `ALTER TABLE users DROP COLUMN allow_downloads`, `DELETE FROM schema_migrations WHERE version='0068_download_preparation'`} {
		if _, err := st.db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	legacy := filepath.Join(t.TempDir(), "legacy.db")
	if err := st.BackupTo(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	safety := filepath.Join(t.TempDir(), "before-upgrade.db")
	called := false
	upgraded, err := Open(ctx, legacy, Options{BeforeMigrate: func(current *Store) error { called = true; return current.BackupTo(ctx, safety) }})
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	if !called {
		t.Fatal("safety backup hook was skipped")
	}
	if err = CheckBackup(ctx, safety); err != nil {
		t.Fatal(err)
	}
	u, err := upgraded.GetUser(ctx, user)
	if err != nil || u == nil || u.AllowDownloads {
		t.Fatal("existing profile or denied-by-default grant changed")
	}
	se, err := upgraded.GetSession(ctx, session)
	if err != nil || se == nil || se.UserID != user {
		t.Fatal("session changed on upgrade")
	}
	it, err := upgraded.GetItem(ctx, item)
	if err != nil || it == nil {
		t.Fatal("item lost on upgrade")
	}
	files, err := upgraded.ItemFiles(ctx, item)
	if err != nil || len(files) != 1 || files[0].ID != f.ID {
		t.Fatal("file identity changed on upgrade")
	}
	if _, _, err = upgraded.CreateDownload(ctx, user, session, item, "after-upgrade", f); err != nil {
		t.Fatal(err)
	}
}

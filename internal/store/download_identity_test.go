package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestDownloadRejectsReusedSourceRevision(t *testing.T) {
	st, user, session, item, f := downloadStoreFixture(t)
	d, _, err := st.CreateDownload(t.Context(), user, session, item, "identity", f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.db.ExecContext(t.Context(), `UPDATE media_files SET cache_key=? WHERE id=?`, NewID(), f.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = st.DownloadSource(t.Context(), d); !errors.Is(err, ErrDownloadNotFound) {
		t.Fatalf("old source identity admitted: %v", err)
	}
	list, err := st.VisibleDownloadRecords(t.Context(), user, session)
	if err != nil || len(list) != 0 {
		t.Fatalf("stale list: %+v %v", list, err)
	}
	if published, err := st.FinishDownload(t.Context(), d, "stale-digest", 1, ""); err != nil || published {
		t.Fatalf("stale source published: %v %v", published, err)
	}
}

func TestDownloadIdentityUpgradeAndOldBackupInvalidateUnboundRecords(t *testing.T) {
	st, user, session, item, f := downloadStoreFixture(t)
	d, _, err := st.CreateDownload(t.Context(), user, session, item, "legacy", f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.FinishDownload(t.Context(), d, "old-digest", 1, ""); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`ALTER TABLE sessions DROP COLUMN is_tool`, `ALTER TABLE downloads DROP COLUMN snapshot`, `ALTER TABLE downloads DROP COLUMN source_cache_key`, `DELETE FROM schema_migrations WHERE version>='0069_download_source_identity'`} {
		if _, err = st.db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	old := filepath.Join(t.TempDir(), "old.db")
	if err = st.BackupTo(t.Context(), old); err != nil {
		t.Fatal(err)
	}
	safety := filepath.Join(t.TempDir(), "safety.db")
	called := false
	upgraded, err := Open(t.Context(), old, Options{BeforeMigrate: func(s *Store) error { called = true; return s.BackupTo(t.Context(), safety) }})
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	if !called {
		t.Fatal("upgrade backup skipped")
	}
	check := func(s *Store) {
		t.Helper()
		got, err := s.Download(t.Context(), user, session, d.ID)
		if err != nil || got.State != "failed" || got.SHA256 != "" || got.SourceCacheKey != "" || got.Revision != d.Revision {
			t.Fatalf("unbound record trusted: %+v %v", got, err)
		}
	}
	check(upgraded)
	// Restore the pre-key safety copy into the upgraded store; migration must run again.
	if err = upgraded.RestoreFrom(t.Context(), safety); err != nil {
		t.Fatal(err)
	}
	check(upgraded)
}

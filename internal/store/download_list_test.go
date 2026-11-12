package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestVisibleDownloadListMatchesPreparationAccess(t *testing.T) {
	for _, scenario := range []string{"visible", "grant", "session", "libraries", "age", "labels", "episode age", "episode labels", "missing", "size", "mtime", "path", "shared", "multipart", "deleted"} {
		t.Run(scenario, func(t *testing.T) {
			st, user, session, item, f := downloadStoreFixture(t)
			ctx := t.Context()
			if _, _, err := st.CreateDownload(ctx, user, session, item, "fixture", f); err != nil {
				t.Fatal(err)
			}
			exec := func(q string, args ...any) {
				t.Helper()
				if _, err := st.db.ExecContext(ctx, q, args...); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "grant":
				exec(`UPDATE users SET allow_downloads=0 WHERE id=?`, user)
			case "session":
				exec(`UPDATE sessions SET user_id=NULL WHERE token_hash=?`, session)
			case "libraries":
				exec(`UPDATE users SET library_ids='0' WHERE id=?`, user)
			case "age":
				exec(`UPDATE users SET max_age=7 WHERE id=?`, user)
				exec(`UPDATE items SET min_age=18 WHERE id=?`, item)
			case "labels":
				exec(`UPDATE users SET exclude_labels='["private"]' WHERE id=?`, user)
				exec(`INSERT INTO item_labels(item_id,label) VALUES(?,'private')`, item)
			case "episode age", "episode labels":
				parent, _, err := st.EnsureItem(ctx, &Item{LibraryID: f.LibraryID, Kind: "series", Title: "Parent"}, "parent")
				if err != nil {
					t.Fatal(err)
				}
				exec(`UPDATE items SET kind='episode',parent_id=? WHERE id=?`, parent, item)
				if scenario == "episode age" {
					exec(`UPDATE users SET max_age=7 WHERE id=?`, user)
					exec(`UPDATE items SET min_age=18 WHERE id=?`, parent)
				} else {
					exec(`UPDATE users SET exclude_labels='["private"]' WHERE id=?`, user)
					exec(`INSERT INTO item_labels(item_id,label) VALUES(?,'private')`, parent)
				}
			case "missing":
				exec(`UPDATE media_files SET missing_since=1 WHERE id=?`, f.ID)
			case "size":
				exec(`UPDATE media_files SET size=size+1 WHERE id=?`, f.ID)
			case "mtime":
				exec(`UPDATE media_files SET mtime=mtime+1 WHERE id=?`, f.ID)
			case "path":
				exec(`UPDATE media_files SET path=path||'changed' WHERE id=?`, f.ID)
			case "shared":
				other, _, err := st.EnsureItem(ctx, &Item{LibraryID: f.LibraryID, Kind: "movie", Title: "Other"}, "other")
				if err != nil {
					t.Fatal(err)
				}
				exec(`INSERT INTO item_files(item_id,file_id,part) VALUES(?,?,0)`, other, f.ID)
			case "multipart":
				exec(`UPDATE item_files SET part=2 WHERE item_id=?`, item)
			case "deleted":
				exec(`DELETE FROM downloads WHERE user_id=?`, user)
			}
			records, err := st.DownloadRecords(ctx, user, session)
			if err != nil {
				t.Fatal(err)
			}
			want := []Download{}
			for _, d := range records {
				if _, err := st.DownloadSource(ctx, &d); err == nil {
					want = append(want, d)
				} else if !errors.Is(err, ErrDownloadNotFound) {
					t.Fatal(err)
				}
			}
			got, err := st.VisibleDownloadRecords(ctx, user, session)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("projection differs from source check: got%+v want%+v", got, want)
			}
			if scenario == "visible" && len(got) != 1 {
				t.Fatal("control fixture is not visible")
			}
			if scenario != "visible" && len(got) != 0 {
				t.Fatal("revoked/changed descriptor visible")
			}
		})
	}
}

func TestVisibleDownloadListReportsDatabaseFailure(t *testing.T) {
	st, user, session, _, _ := downloadStoreFixture(t)
	st.Close()
	if _, err := st.VisibleDownloadRecords(context.Background(), user, session); err == nil {
		t.Fatal("database failure hidden as empty list")
	}
}

package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ams/internal/activity"
	"ams/internal/auth"
	"ams/internal/backup"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
)

func TestEmptyTrashAccessAndRealSafetyBackup(t *testing.T) {
	for _, failBackup := range []bool{false, true} {
		t.Run(fmt.Sprint(failBackup), func(t *testing.T) {
			s, viewer := securityServer(t)
			ctx := t.Context()
			owner, err := s.store.CreateUser(ctx, "Owner", true, "blue", "", "")
			if err != nil {
				t.Fatal(err)
			}
			for token, id := range map[string]string{"viewer": viewer.ID, "owner": owner.ID} {
				if err := s.store.CreateSession(ctx, auth.HashToken(token), id, "password", "fixture"); err != nil {
					t.Fatal(err)
				}
			}
			lib, err := s.store.SyncLibrary(ctx, "Trash", "movies", "tmdb_aired", []string{t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Gone"}, "gone")
			if err != nil {
				t.Fatal(err)
			}
			file, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: "/fixture/gone.mkv", Size: 1, ModTime: 1}, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.store.SetFileItems(ctx, file, []string{id}, 0); err != nil {
				t.Fatal(err)
			}
			if _, err := s.store.MarkMissing(ctx, lib, 2, 3); err != nil {
				t.Fatal(err)
			}
			s.scanner = scanner.New(ctx, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
			s.scanner.Activity = activity.New()
			backupDir := filepath.Join(t.TempDir(), "backups")
			if failBackup {
				if err := os.WriteFile(backupDir, []byte("not a directory"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			s.backups = backup.New(backupDir, s.store, s.log)
			// Batch cleanup never falls back to individual confirmation: without fresh
			// scan proof it skips, even if the request is from an administrator.
			for _, token := range []string{"viewer", "owner"} {
				r := httptest.NewRequest("POST", fmt.Sprintf("/api/libraries/%d/empty-trash?safe=1", lib), nil)
				r.SetPathValue("id", fmt.Sprint(lib))
				r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
				w := httptest.NewRecorder()
				s.adminOnly(s.emptyTrash)(w, r)
				want := http.StatusForbidden
				if token == "owner" {
					want = http.StatusOK
				}
				if w.Code != want || (token == "owner" && !strings.Contains(w.Body.String(), `"skipped":"scan_required"`)) {
					t.Fatalf("safe cleanup %s: %d %s", token, w.Code, w.Body.String())
				}
			}
			if item, err := s.store.GetItem(ctx, id); err != nil || item == nil {
				t.Fatalf("unsafe batch deleted item: %v %v", item, err)
			}

			for _, token := range []string{"viewer", "owner"} {
				r := httptest.NewRequest("POST", fmt.Sprintf("/api/libraries/%d/empty-trash", lib), nil)
				r.SetPathValue("id", fmt.Sprint(lib))
				r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
				w := httptest.NewRecorder()
				s.adminOnly(s.emptyTrash)(w, r)
				want := 403
				if token == "owner" {
					want = 200
					if failBackup {
						want = 500
					}
				}
				if w.Code != want {
					t.Fatalf("%s status=%d want=%d %s", token, w.Code, want, w.Body.String())
				}
				if token == "owner" && failBackup && !strings.Contains(w.Body.String(), "nothing was removed") {
					t.Fatalf("backup failure lost its recovery explanation: %s", w.Body.String())
				}
			}
			item, err := s.store.GetItem(ctx, id)
			if err != nil || (item != nil) != failBackup {
				t.Fatalf("item=%v err=%v", item, err)
			}
			if failBackup {
				return
			}
			copies, err := s.backups.List()
			if err != nil || len(copies) != 1 || copies[0].Reason != backup.ReasonBeforeDelete {
				t.Fatalf("backups=%v %v", copies, err)
			}
			restored, err := store.Open(context.Background(), filepath.Join(backupDir, copies[0].Name), store.Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer restored.Close()
			original, err := restored.GetItem(ctx, id)
			if err != nil || original == nil {
				t.Fatalf("backup did not precede deletion: %v %v", original, err)
			}
		})
	}
}

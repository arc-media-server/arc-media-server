package api

import (
	"ams/internal/activity"
	"ams/internal/backup"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSafetyBackupWindowCannotAdmitANewScan(t *testing.T) {
	for _, operation := range []string{"restore", "delete"} {
		t.Run(operation, func(t *testing.T) {
			s, _ := securityServer(t)
			root := t.TempDir()
			lib, err := s.store.SyncLibrary(t.Context(), "Fixture", "movies", "tmdb_aired", []string{root})
			if err != nil {
				t.Fatal(err)
			}
			life, cancel := context.WithCancel(t.Context())
			defer cancel()
			scan := scanner.New(life, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
			defer func() { cancel(); scan.Wait() }()
			tasks := activity.New()
			scan.Activity = tasks
			s.scanner = scan
			dir := t.TempDir()
			s.backups = backup.New(dir, s.store, s.log)
			s.backups.Activity = tasks
			saved, err := s.backups.Create(t.Context(), backup.ReasonManual)
			if err != nil {
				t.Fatal(err)
			}
			reason := backup.ReasonBeforeRestore
			if operation == "delete" {
				reason = backup.ReasonBeforeDelete
			}
			// Occupy only scratch names to hold the real safety-backup collision wait, not a test hook.
			now := time.Now()
			for offset := 0; offset < 3; offset++ {
				name := fmt.Sprintf("ams-%s-%s.db", now.Add(time.Duration(offset)*time.Second).Format("20060102-150405"), reason)
				if err := os.WriteFile(filepath.Join(dir, name), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			request := httptest.NewRequest("POST", "http://arc/api/backups/fixture/restore", nil)
			request.SetPathValue("name", saved.Name)
			request.SetPathValue("id", fmt.Sprint(lib))
			response := httptest.NewRecorder()
			done := make(chan struct{})
			go func() {
				defer close(done)
				if operation == "restore" {
					s.restoreBackup(response, request)
				} else {
					s.deleteLibrary(response, request)
				}
			}()
			deadline := time.Now().Add(5 * time.Second)
			entered := false
			for time.Now().Before(deadline) {
				running, _ := tasks.Snapshot()
				for _, task := range running {
					if task.Kind == "backup" {
						entered = true
					}
				}
				if entered {
					break
				}
				select {
				case <-done:
					t.Fatalf("%s finished before safety-backup checkpoint: %d %s", operation, response.Code, response.Body.String())
				case <-time.After(time.Millisecond):
				}
			}
			if !entered {
				t.Fatal("safety backup never started")
			}
			if scan.Start(lib) {
				t.Errorf("%s admitted discovery during its safety backup", operation)
				scan.Stop(lib, scan.Status(lib).JobID)
			}
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("mutation did not finish")
			}
			if response.Code != 204 {
				t.Fatalf("mutation status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

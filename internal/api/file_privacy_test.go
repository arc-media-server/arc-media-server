package api

import (
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileFilePathsAreFilenamesAndAdminPathsStayFull(t *testing.T) {
	for _, windows := range []bool{false, true} {
		t.Run(fmt.Sprint(windows), func(t *testing.T) {
			s, u := securityServer(t)
			life, cancel := context.WithCancel(t.Context())
			defer func() { cancel(); s.scanner.Wait() }()
			s.scanner = scanner.New(life, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
			root := filepath.Join(t.TempDir(), "private-root")
			lib, err := s.store.SyncLibrary(t.Context(), "Fixture", "movies", "tmdb_aired", []string{root})
			if err != nil {
				t.Fatal(err)
			}
			id, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "privacy")
			if err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(root, "Film.mkv")
			if windows {
				source = `C:\owner\private\Film.mkv`
			}
			file := store.MediaFile{LibraryID: lib, Path: source, Size: 1, ModTime: 1, ProbeError: "could not inspect " + source}
			file.ID, err = s.store.SaveFile(t.Context(), &file, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.store.SetFileItems(t.Context(), file.ID, []string{id}, 0); err != nil {
				t.Fatal(err)
			}
			// Exercise unavailable-root handling without reading any real media.
			s.scanner.Start(lib)
			s.scanner.Wait()
			for _, role := range []string{"admin", "household", "friend", "admin again"} {
				profile := *u
				profile.IsAdmin = strings.HasPrefix(role, "admin")
				profile.IsFriend = role == "friend"
				request := func(method, body string) *http.Request {
					r := httptest.NewRequest(method, "/", strings.NewReader(body))
					r.SetPathValue("id", id)
					return r.WithContext(context.WithValue(r.Context(), userKey, &profile))
				}
				for _, batch := range []bool{false, true} {
					r := request("GET", "")
					w := httptest.NewRecorder()
					if batch {
						r = request("POST", fmt.Sprintf(`{"ids":[%q]}`, id))
						s.batchItems(w, r)
					} else {
						s.getItem(w, r)
					}
					if w.Code != 200 {
						t.Fatalf("%s details: %d %s", role, w.Code, w.Body.String())
					}
					var dto itemDTO
					if batch {
						var result itemBatch
						if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || len(result.Items) != 1 {
							t.Fatalf("batch: %s %v", w.Body.String(), err)
						}
						dto = result.Items[0]
					} else if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
						t.Fatal(err)
					}
					if len(dto.Files) != 1 {
						t.Fatal("file metadata vanished")
					}
					want := "Film.mkv"
					if profile.IsAdmin {
						want = source
					}
					if dto.Files[0].Path != want {
						t.Errorf("%s batch=%v path=%q want %q", role, batch, dto.Files[0].Path, want)
					}
					if !profile.IsAdmin && strings.Contains(dto.Files[0].ProbeError, source) {
						t.Errorf("%s probe error exposed full source", role)
					}
				}
				w := httptest.NewRecorder()
				s.listLibraries(w, request("GET", ""))
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
				if !profile.IsAdmin && strings.Contains(w.Body.String(), root) {
					t.Errorf("%s library roots/scan error exposed host path", role)
				}
				r := request("GET", "")
				r.SetPathValue("id", fmt.Sprint(lib))
				w = httptest.NewRecorder()
				s.scanStatus(w, r)
				if !profile.IsAdmin && strings.Contains(w.Body.String(), root) {
					t.Errorf("%s scan error exposed host path", role)
				}
			}
			original, err := s.store.GetFileByPath(t.Context(), source)
			if err != nil || original == nil || original.ID != file.ID || original.Path != source {
				t.Fatalf("response masking changed internal playback path: %+v %v", original, err)
			}
		})
	}
}

func TestProfileScanErrorDetailsStayWithAdministrator(t *testing.T) {
	original := scanner.Status{Error: "cannot open /private/server/root: permission denied", FilesSeen: 7, ProbeErrors: 2}
	for _, admin := range []bool{false, true} {
		profile := &store.User{IsAdmin: admin}
		r := httptest.NewRequest("GET", "/", nil)
		r = r.WithContext(context.WithValue(r.Context(), userKey, profile))
		got := profileScanStatus(r, original)
		if admin && got.Error != original.Error {
			t.Fatal("administrator diagnostics changed")
		}
		if !admin && strings.Contains(got.Error, "/private/") {
			t.Fatal("scan diagnostic exposed host root")
		}
		if got.FilesSeen != 7 || got.ProbeErrors != 2 {
			t.Fatal("progress counts changed")
		}
	}
}

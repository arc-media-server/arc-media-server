package api

import (
	"ams/internal/auth"
	"ams/internal/settings"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPrerollCannotExposeConfiguredNonVideoOrLinkedPrivateFile(t *testing.T) {
	s, u := securityServer(t)
	root := t.TempDir()
	private := filepath.Join(root, "private.json")
	if err := os.WriteFile(private, []byte("private fixture bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked.mp4")
	linkErr := os.Symlink(private, link)
	configured := private
	s.settings = func() settings.Settings { return settings.Settings{PrerollPaths: configured} }
	if err := s.store.CreateSession(t.Context(), auth.HashToken("preroll-fixture"), u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	candidates := []string{private}
	if linkErr == nil {
		candidates = append(candidates, link)
	}
	for _, prefix := range []string{"/api", "/api/v1"} {
		for _, candidate := range candidates {
			configured = candidate
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", prefix+"/preroll/0", nil)
			r.Header.Set("Authorization", "Bearer preroll-fixture")
			h.ServeHTTP(w, r)
			if w.Code != 404 || w.Body.String() == "private fixture bytes" {
				t.Errorf("configured %s exposed through %s: status %d body=%q", filepath.Base(candidate), prefix, w.Code, w.Body.String())
			}
		}
	}
	// A configured video outside every library remains an intentional server-wide pre-roll.
	clip := filepath.Join(root, "fixture.mp4")
	if err := os.WriteFile(clip, []byte("configured clip bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	configured = clip
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/preroll/0", nil)
	r.Header.Set("Authorization", "Bearer preroll-fixture")
	h.ServeHTTP(w, r)
	if w.Code != 200 || w.Body.String() != "configured clip bytes" {
		t.Fatalf("explicit outside-library clip stopped working: %d", w.Code)
	}
}

func TestPrerollFolderOnlyIncludesRegularVideoFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"one.mp4", "two.MKV", "private.db", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "directory.mp4"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "private.db"), filepath.Join(root, "linked.mov")); err != nil {
		t.Log("symlink fixture unavailable:", err)
	}
	files := prerollFiles(root)
	if len(files) != 2 {
		t.Fatalf("folder included non-video/linked entries: %v", files)
	}
}

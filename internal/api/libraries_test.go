package api

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLibraryFolderValidationWithoutPreview(t *testing.T) {
	s, _ := securityServer(t)
	root := t.TempDir()
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		paths []string
		valid bool
	}{
		{"readable folder", []string{root}, true},
		{"missing folder", []string{filepath.Join(root, "missing")}, false},
		{"regular file", []string{file}, false},
		{"no folders", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := libraryBody{Name: "Fixture", Kind: "movies", Paths: tc.paths}
			err := s.validate(context.Background(), &b, 0)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v: %v", tc.valid, err)
			}
		})
	}
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return // Unix permission bits cannot demonstrate access denial here.
	}
	locked := t.TempDir()
	if err := os.Chmod(locked, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0700) })
	if err := s.validate(context.Background(), &libraryBody{Kind: "shows", Paths: []string{locked}}, 0); err == nil {
		t.Fatal("unreadable folder accepted")
	}
}

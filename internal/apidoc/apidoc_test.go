package apidoc

import (
	"os"
	"path/filepath"
	"testing"
)

// docs/API.md must match the code: run `go run ./tools/apidoc` after changing the API.
func TestDocsUpToDate(t *testing.T) {
	root := filepath.Join("..", "..")
	want, err := Generate(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "API.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatal("docs/API.md is out of date: run `go run ./tools/apidoc`")
	}
}

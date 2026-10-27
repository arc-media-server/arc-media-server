package apidoc

import (
	"os"
	"path/filepath"
	"regexp"
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

// docs/openapi.json must match the code too. Generating it also fails when a route's handler
// answers in a way the generator can't see (no writeJSON or WriteHeader), so every new endpoint
// is documented.
func TestOpenAPIUpToDate(t *testing.T) {
	root := filepath.Join("..", "..")
	want, err := GenerateOpenAPI(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "docs", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatal("docs/openapi.json is out of date: run `go run ./tools/apidoc`")
	}
}

// The API docs describe Arc in its own terms, not by comparison with other media servers
// ("like Plex's ..."). Naming a server Arc imports from is fine.
func TestDocsDoNotCompareWithPlex(t *testing.T) {
	root := filepath.Join("..", "..")
	compare := regexp.MustCompile(`(?i)\b(like|as|as in|than)( in)? plex|plex's`)
	for _, name := range []string{"API.md", "openapi.json"} {
		data, err := os.ReadFile(filepath.Join(root, "docs", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range compare.FindAllIndex(data, -1) {
			t.Errorf("docs/%s compares with Plex: %q", name, data[max(0, m[0]-60):min(len(data), m[1]+40)])
		}
	}
}

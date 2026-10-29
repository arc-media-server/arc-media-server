package apidoc

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// The v1 promise: nothing in docs/api-v1-contract.txt may disappear or change type, and the file
// records everything the code answers with (so new fields are protected from then on).
func TestV1ContractKept(t *testing.T) {
	recorded, current, err := contract(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, l := range current {
		have[l] = true
	}
	var broken []string
	for _, l := range recorded {
		if !have[l] {
			broken = append(broken, l)
		}
	}
	if len(broken) > 0 {
		t.Fatalf("these /api/v1 promises are broken (apps may rely on them); restore them, or delete the lines from docs/api-v1-contract.txt only if the break is deliberate:\n%s", strings.Join(broken, "\n"))
	}
	known := map[string]bool{}
	for _, l := range recorded {
		known[l] = true
	}
	for _, l := range current {
		if !known[l] {
			t.Fatalf("docs/api-v1-contract.txt is out of date (%q is new): run `go run ./tools/apidoc`", l)
		}
	}
}

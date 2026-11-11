package apidoc

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func developerDocument(t *testing.T) map[string]any {
	t.Helper()
	data, err := GenerateOpenAPI("../..")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestProfileSchemasDescribeExistingWireBehavior(t *testing.T) {
	doc := developerDocument(t)
	components := doc["components"].(map[string]any)["schemas"].(map[string]any)
	profile := components["Profile"].(map[string]any)["properties"].(map[string]any)["library_ids"].(map[string]any)
	if got := schemaKinds(profile, nil, nil); !slices.Equal(got, []string{"array", "null"}) {
		t.Errorf("all-library null sentinel missing: %v", got)
	}
	labels := components["Profile"].(map[string]any)["properties"].(map[string]any)["exclude_labels"].(map[string]any)
	if got := schemaKinds(labels, nil, nil); !slices.Equal(got, []string{"array", "null"}) {
		t.Errorf("new-profile label response can be null: %v", got)
	}
	resolve := func(s map[string]any) map[string]any {
		if ref, ok := s["$ref"].(string); ok {
			return components[strings.TrimPrefix(ref, "#/components/schemas/")].(map[string]any)
		}
		return s
	}
	for _, tc := range []struct {
		path, method string
		wantName     bool
	}{{"/profiles", "post", true}, {"/profiles/{id}", "patch", false}} {
		op := doc["paths"].(map[string]any)[tc.path].(map[string]any)[tc.method].(map[string]any)
		s := resolve(op["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any))
		required, _ := s["required"].([]any)
		if slices.Contains(required, any("name")) != tc.wantName {
			t.Errorf("%s name requirement: %v", tc.method, required)
		}
		for _, field := range []string{"friend", "library_ids", "color", "pin", "max_age", "exclude_labels"} {
			if slices.Contains(required, any(field)) {
				t.Errorf("%s wrongly requires %s", tc.method, field)
			}
		}
		ids := s["properties"].(map[string]any)["library_ids"].(map[string]any)
		if got := schemaKinds(ids, nil, nil); !slices.Equal(got, []string{"array", "null"}) {
			t.Errorf("%s library selection input: %v", tc.method, got)
		}
		description, _ := ids["description"].(string)
		if !strings.Contains(description, "all") {
			t.Errorf("%s library selection has no semantics", tc.method)
		}
	}
}

func TestViewerIntroductionLinksToServedGuide(t *testing.T) {
	description := developerDocument(t)["info"].(map[string]any)["description"].(string)
	if !strings.Contains(description, "(/docs/client-integration.html)") {
		t.Error("viewer guide link does not target a bundled HTML page")
	}
	if strings.Contains(description, "](CLIENT-INTEGRATION.md)") {
		t.Error("file-relative Markdown link leaks into served documentation")
	}
}

func TestContractProtectsArrayAndNullableAlternatives(t *testing.T) {
	fixture := func(schema string) []byte {
		return []byte(`{"paths":{"/profiles":{"get":{"responses":{"200":{"content":{"application/json":{"schema":{"type":"object","properties":{"library_ids":` + schema + `}}}}}}}}}}`)
	}
	array := `{"type":"array","items":{"type":"integer"}}`
	nullable := `{"anyOf":[` + array + `,{"type":"null"}]}`
	old, err := contractLines(fixture(array))
	if err != nil {
		t.Fatal(err)
	}
	current, err := contractLines(fixture(nullable))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range old {
		if !slices.Contains(current, line) {
			t.Errorf("documenting existing null erased an array promise: %s", line)
		}
	}
	for _, line := range []string{"GET /profiles 200 .library_ids array|null", "GET /profiles 200 .library_ids null"} {
		if !slices.Contains(current, line) {
			t.Errorf("nullable alternative not protected: %s", line)
		}
	}
	changed, err := contractLines(fixture(strings.Replace(nullable, `"integer"`, `"string"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(changed, "GET /profiles 200 .library_ids[] integer") {
		t.Error("retyping array items still satisfies old promise")
	}
	narrowed, err := contractLines(fixture(array))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(narrowed, "GET /profiles 200 .library_ids array|null") || slices.Contains(narrowed, "GET /profiles 200 .library_ids null") {
		t.Error("removing null still satisfies nullable promises")
	}
	removed, err := contractLines(fixture(`{"type":"null"}`))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(removed, "GET /profiles 200 .library_ids array") {
		t.Error("removing array still satisfies array promise")
	}
}

func TestClientGuideJSONExamplesAreValidAndNamed(t *testing.T) {
	data, err := os.ReadFile("../../docs/CLIENT-INTEGRATION.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	examples := regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n```").FindAllStringSubmatch(text, -1)
	if len(examples) < 5 {
		t.Fatal("canonical client walkthrough has no complete JSON examples")
	}
	for _, example := range examples {
		if !json.Valid([]byte(example[1])) {
			t.Errorf("invalid JSON example: %s", example[1])
		}
	}
	for _, name := range []string{"device-login", "select-profile", "device-link", "playback", "progress"} {
		if !strings.Contains(text, "<!-- example: "+name+" -->") {
			t.Errorf("missing reusable example %s", name)
		}
	}
}

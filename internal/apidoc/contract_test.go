package apidoc

import (
	"slices"
	"strings"
	"testing"
)

func TestContractTracksNestedResponseFieldsAndSuccessStatuses(t *testing.T) {
	const fixture = `{"paths":{"/library":{"get":{"responses":{"200":{"content":{"application/json":{"schema":{"type":"object","properties":{"rows":{"type":"array","items":{"$ref":"#/components/schemas/Row"}}}}}}},"403":{"content":{"application/json":{"schema":{"type":"object","properties":{"error":{"type":"string"}}}}}}}}}},"components":{"schemas":{"Row":{"type":"object","properties":{"title":{"type":"string"},"count":{"type":"integer"}}}}}}`
	lines, err := contractLines([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"GET /library", "GET /library 200", "GET /library 200 .rows array", "GET /library 200 .rows[] object", "GET /library 200 .rows[].title string", "GET /library 200 .rows[].count integer"} {
		if !slices.Contains(lines, want) {
			t.Errorf("missing promise %q in %v", want, lines)
		}
	}
	for _, line := range lines {
		if strings.Contains(line, "403") || strings.Contains(line, ".error") {
			t.Errorf("failure response was frozen as a success: %s", line)
		}
	}
	changed, err := contractLines([]byte(strings.Replace(fixture, `"title"`, `"heading"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(changed, "GET /library 200 .rows[].title string") {
		t.Fatal("renamed field still satisfies the old promise")
	}
	if !slices.Contains(changed, "GET /library 200 .rows[].heading string") {
		t.Fatal("new field was not recorded")
	}
	retyped, err := contractLines([]byte(strings.Replace(fixture, `"count":{"type":"integer"}`, `"count":{"type":"string"}`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(retyped, "GET /library 200 .rows[].count integer") {
		t.Fatal("retyped field still satisfies the old promise")
	}
	if !slices.Contains(retyped, "GET /library 200 .rows[].count string") {
		t.Fatal("new field type was not recorded")
	}
}

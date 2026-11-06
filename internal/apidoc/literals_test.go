package apidoc

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"testing"
)

func TestLiteralEnvelopesMergeBranchesAndPreserveNull(t *testing.T) {
	source := `package fixture
 func envelope(){names:=[]string{"A"};_=map[string]any{"names":names,"parent":nil,"path":""};var parent *string;_=map[string]any{"names":names,"parent":parent,"path":"x","optional":true}}
 `
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	if _, err = (&types.Config{}).Check("fixture", fset, []*ast.File{file}, info); err != nil {
		t.Fatal(err)
	}
	values := []ast.Expr{}
	ast.Inspect(file, func(node ast.Node) bool {
		if literal, ok := node.(*ast.CompositeLit); ok {
			if _, ok := info.TypeOf(literal).Underlying().(*types.Map); ok {
				values = append(values, literal)
			}
		}
		return true
	})
	s := &schemas{components: map[string]any{}, names: map[*types.TypeName]string{}}
	out := responseLiterals(info, s, values)
	if out == nil {
		t.Fatal("literal envelope lost")
	}
	props := out["properties"].(map[string]any)
	if got := schemaKinds(props["names"].(map[string]any), nil, nil); !slices.Equal(got, []string{"array", "null"}) {
		t.Fatal("nil slice misdescribed", got)
	}
	if got := schemaKinds(props["parent"].(map[string]any), nil, nil); !slices.Equal(got, []string{"null", "string"}) {
		t.Fatal("nullable branch lost", got)
	}
	required := out["required"].([]string)
	if slices.Contains(required, "optional") || !slices.Contains(required, "names") {
		t.Fatal("branch required keys", required)
	}
}
func TestContractKeepsNullableChildrenAndDynamicValues(t *testing.T) {
	schema := `{"paths":{"/example":{"get":{"responses":{"200":{"content":{"application/json":{"schema":{"type":"object","properties":{"items":{"anyOf":[{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}},{"type":"null"}]},"features":{"type":"object","additionalProperties":{"type":"object","properties":{"supported":{"type":"boolean"}}}}}}}}}}}}}}`
	lines, err := contractLines([]byte(schema))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"GET /example 200 .items array|null", "GET /example 200 .items[].id string", "GET /example 200 .features{}.supported boolean"} {
		if !slices.Contains(lines, line) {
			t.Fatal("missing guarantee", line, lines)
		}
	}
}
func TestPublishedFixedEnvelopeSchemasHaveTheirFields(t *testing.T) {
	data, err := GenerateOpenAPI("../..")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	paths := doc["paths"].(map[string]any)
	for _, tc := range []struct {
		path   string
		fields []string
	}{{"/optimized", []string{"versions", "total_size"}}, {"/cast", []string{"url", "local_only"}}, {"/markers", nil}} {
		if tc.fields == nil {
			continue
		}
		method := "get"
		if tc.path == "/cast" {
			method = "post"
		}
		response := paths[tc.path].(map[string]any)[method].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
		properties := response["properties"].(map[string]any)
		for _, field := range tc.fields {
			if _, ok := properties[field]; !ok {
				t.Errorf("%s missing %s", tc.path, field)
			}
		}
	}
}

func TestSuccessJSONEnvelopesCannotSilentlyBecomeUnstructured(t *testing.T) {
	data, err := GenerateOpenAPI("../..")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Content map[string]struct{ Schema map[string]any }
			}
		}
		Components struct{ Schemas map[string]map[string]any }
	}
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for path, methods := range doc.Paths {
		for method, op := range methods {
			for status, response := range op.Responses {
				if len(status) == 0 || status[0] != '2' {
					continue
				}
				body, ok := response.Content["application/json"]
				if !ok {
					continue
				}
				schema := body.Schema
				if ref, ok := schema["$ref"].(string); ok {
					schema = doc.Components.Schemas[ref[len("#/components/schemas/"):]]
				}
				if len(schema) == 0 {
					t.Errorf("%s %s %s has no JSON shape", method, path, status)
					continue
				}
				if schema["type"] == "object" && schema["properties"] == nil {
					value, _ := schema["additionalProperties"].(map[string]any)
					if len(value) == 0 {
						t.Errorf("%s %s %s has an unstructured JSON envelope; use a DTO or fixed literal", method, path, status)
					}
				}
			}
		}
	}
}

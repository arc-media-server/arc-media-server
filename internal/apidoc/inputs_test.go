package apidoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestRoutesDoNotLoseDecoratedHandlers(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "api.go")
	code := `package api
func register() {
 mux.HandleFunc("PATCH /api/settings", admin(s.logSettingChanges(s.updateSettings)))
 mux.HandleFunc("GET /api/health", s.health)
 mux.HandleFunc("/api/", func(){})
}`
	if err := os.WriteFile(file, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := routes(dir)
	if err != nil || len(got) != 2 {
		t.Fatalf("routes=%v err=%v", got, err)
	}
	if got[0].handler != "updateSettings" || got[0].access != "admin" {
		t.Fatalf("lost wrapper semantics: %+v", got[0])
	}
	code = `package api; func register(){ mux.HandleFunc("GET /api/new", unknown(s.health)) }`
	if err := os.WriteFile(file, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := routes(dir); err == nil {
		t.Fatal("unknown wrapper silently accepted")
	}
}

func TestGeneratedInputsDescribeActualPartialRequests(t *testing.T) {
	data, err := os.ReadFile("../../docs/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	paths := doc["paths"].(map[string]any)
	components := doc["components"].(map[string]any)["schemas"].(map[string]any)
	resolve := func(schema map[string]any) map[string]any {
		if ref, ok := schema["$ref"].(string); ok {
			return components[ref[len("#/components/schemas/"):]].(map[string]any)
		}
		return schema
	}
	input := func(path, method string) map[string]any {
		t.Helper()
		return resolve(paths[path].(map[string]any)[method].(map[string]any)["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any))
	}
	required := func(s map[string]any, field string) bool {
		list, _ := s["required"].([]any)
		return slices.Contains(list, any(field))
	}
	patch := input("/items/{id}", "patch")
	if required(patch, "overview") || required(patch, "title") {
		t.Fatal("partial edit fields are required")
	}
	profile := input("/auth/profile", "post")
	if !required(profile, "id") || required(profile, "pin") || required(profile, "password") {
		t.Fatal("profile conditional credentials incorrect")
	}
	playback := input("/items/{id}/playback", "post")
	if !required(playback, "caps") || required(playback, "file_id") || required(playback, "audio_stream") {
		t.Fatal("playback defaults not optional")
	}
	for _, tc := range []struct{ path, field string }{{"/items/{id}/artwork", "path"}, {"/series/{id}/display-ordering", "ordering_id"}} {
		field := input(tc.path, "put")["properties"].(map[string]any)[tc.field].(map[string]any)
		alternatives, _ := field["anyOf"].([]any)
		nullable := false
		for _, v := range alternatives {
			if v.(map[string]any)["type"] == "null" {
				nullable = true
			}
		}
		if !nullable {
			t.Fatalf("%s reset doesn't allow null", tc.path)
		}
	}
	settings := input("/settings", "patch")
	if _, ok := settings["properties"].(map[string]any)["scan_automatic"]; !ok {
		t.Fatal("settings typed request missing")
	}
	// Required output properties retain the original v1 encoding contract.
	response := components["PlaybackPlan"].(map[string]any)
	if !required(response, "file_id") || !required(response, "method") {
		t.Fatal("request rules weakened response contract")
	}
}

package apidoc

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Check the published document, rather than reproducing its metadata implementation.
func TestPublishedClientWireContracts(t *testing.T) {
	data, err := os.ReadFile("../../docs/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	paths := doc["paths"].(map[string]any)
	operation := func(path, method string) map[string]any {
		t.Helper()
		return paths[path].(map[string]any)[method].(map[string]any)
	}
	parameter := func(op map[string]any, name, in string) map[string]any {
		t.Helper()
		for _, p := range op["parameters"].([]any) {
			v := p.(map[string]any)
			if v["name"] == name && v["in"] == in {
				return v
			}
		}
		t.Fatalf("missing %s parameter %s", in, name)
		return nil
	}
	items := operation("/items", "get")
	limit := parameter(items, "limit", "query")["schema"].(map[string]any)
	if limit["type"] != "integer" || limit["default"] != float64(100) || limit["maximum"] != float64(500) {
		t.Fatal(limit)
	}
	reverse := parameter(items, "reverse", "query")["schema"].(map[string]any)
	if reverse["type"] != "string" || reverse["x-arc-logical-type"] != "boolean" {
		t.Fatal(reverse)
	}
	events := operation("/events", "get")
	if parameter(events, "Last-Event-ID", "header")["schema"].(map[string]any)["maxLength"] != float64(80) {
		t.Fatal("missing cursor bound")
	}
	if events["x-arc-sse-events"] == nil {
		t.Fatal("missing typed event payloads")
	}
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	hint, _ := json.Marshal(schemas["EventHint"])
	if strings.Contains(string(hint), "user_id") || strings.Contains(string(hint), "admin") || !strings.Contains(string(hint), "library_id") {
		t.Fatal(string(hint))
	}
	errors := schemas["Error"].(map[string]any)["properties"].(map[string]any)
	if errors["code"] == nil || errors["request_id"] == nil || errors["needs"] == nil {
		t.Fatal(errors)
	}
	palette := operation("/items/{id}/artwork/palette", "get")
	parameter(palette, "If-None-Match", "header")
	if palette["responses"].(map[string]any)["304"] == nil {
		t.Fatal("missing palette 304")
	}

	lookup := operation("/items/lookup", "get")
	if parameter(lookup, "provider", "query")["required"] != true || parameter(lookup, "provider_id", "query")["required"] != true {
		t.Fatal("lookup identity is required")
	}
	response := func(path, method, status string) map[string]any {
		return operation(path, method)["responses"].(map[string]any)[status].(map[string]any)
	}
	for _, tc := range []struct{ path, media string }{
		{"/items/{id}/subtitles/{track}/vtt", "text/vtt"},
		{"/items/{id}/subtitles/{track}/ass", "text/x-ssa"},
		{"/transcode/{sid}/{file}", "application/vnd.apple.mpegurl"},
		{"/items/{id}/stream", "video/mp4"},
	} {
		if response(tc.path, "get", "200")["content"].(map[string]any)[tc.media] == nil {
			t.Errorf("missing %s for %s", tc.media, tc.path)
		}
	}
	for _, p := range operation("/items/{id}/subtitles/{track}/vtt", "get")["parameters"].([]any) {
		if p.(map[string]any)["name"] == "Range" {
			t.Fatal("streaming WebVTT cannot seek byte ranges")
		}
	}
	parameter(operation("/items/{id}/stream", "get"), "Range", "header")
	if response("/items/{id}/stream", "get", "206")["headers"].(map[string]any)["Content-Range"] == nil {
		t.Fatal("missing partial range response")
	}
	upload := operation("/items/{id}/artwork/{type}", "post")["requestBody"].(map[string]any)
	if upload["x-arc-max-bytes"] != float64(20971520) || upload["content"].(map[string]any)["image/webp"] == nil {
		t.Fatal("missing raw image upload contract")
	}
	for path, value := range paths {
		for method, value := range value.(map[string]any) {
			op := value.(map[string]any)
			for status, response := range op["responses"].(map[string]any) {
				if response.(map[string]any)["headers"].(map[string]any)["X-Request-ID"] == nil {
					t.Errorf("%s %s %s missing correlation", method, path, status)
				}
			}
		}
	}
	if _, err := queryParameter("newHandler", "unknown"); err == nil {
		t.Fatal("unknown query parameter silently documented")
	}
}

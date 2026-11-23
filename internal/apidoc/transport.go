package apidoc

// supplementOperation describes wire contracts which cannot be recovered from Go JSON types.
// Keep these next to the generator so regeneration preserves transport details.
func supplementOperation(r route, op map[string]any, components map[string]any) {
	if r.access != "anyone" && (r.method == "POST" || r.method == "PUT" || r.method == "PATCH" || r.method == "DELETE") {
		if body, ok := op["requestBody"].(map[string]any); ok {
			if content, ok := body["content"].(map[string]any); ok && content["application/json"] != nil {
				body["description"] = "Authenticated JSON reads have a 2 MiB server ceiling; endpoint-specific limits can be smaller. Known oversized Content-Length returns 413; streamed overflows can return 400 from the decoder."
				body["x-arc-server-max-bytes"] = 2097152
			}
		}
	}
	responses := op["responses"].(map[string]any)
	header := func(description string) map[string]any {
		return map[string]any{"description": description, "schema": map[string]any{"type": "string"}}
	}
	parameter := func(name, description string) {
		params, _ := op["parameters"].([]any)
		op["parameters"] = append(params, map[string]any{"name": name, "in": "header", "required": false, "description": description, "schema": map[string]any{"type": "string"}})
	}
	content := func(types ...string) map[string]any {
		result := map[string]any{}
		for _, typ := range types {
			result[typ] = map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}
		}
		return result
	}
	media := func(types ...string) {
		responses["200"] = map[string]any{"description": "The file or stream; media type depends on the selected resource", "content": content(types...)}
	}
	ranges := func() {
		parameter("Range", "Byte range for file responses. Unsupported/unsatisfiable ranges can return plain-text 416.")
		parameter("If-Range", "Send the range only when the file validator matches; otherwise receive the full file.")
		parameter("If-Modified-Since", "Return 304 when the selected local file has not changed.")
		full := responses["200"].(map[string]any)
		responses["206"] = map[string]any{"description": "Partial file response", "content": full["content"], "headers": map[string]any{"Content-Range": header("Returned byte range and total size"), "Accept-Ranges": header("bytes")}}
		responses["304"] = map[string]any{"description": "Not modified; no response body"}
		responses["416"] = map[string]any{"description": "Unsatisfiable byte range; transport error is plain text", "content": map[string]any{"text/plain": map[string]any{"schema": map[string]any{"type": "string"}}}, "headers": map[string]any{"Content-Range": header("For unsatisfiable byte ranges: bytes */size")}}
	}
	switch r.handler {
	case "uploadClientLogs":
		body := op["requestBody"].(map[string]any)
		body["description"] = "User-approved app events only. Use the device Bearer token with a selected profile. Never send raw logs or arbitrary strings. See docs/CLIENT-LOGS.md for the event vocabulary."
		body["x-arc-server-max-bytes"] = 262144
		responses["400"] = map[string]any{"description": "Invalid event batch or unknown field"}
		responses["403"] = map[string]any{"description": "An app session is required; browser and tool sessions cannot upload"}
		responses["413"] = map[string]any{"description": "Batch exceeds 256 KiB"}
	case "getClientLogs":
		responses["404"] = map[string]any{"description": "Device log selection expired or was revoked"}
	case "createProfile", "updateProfile":
		// RawMessage represents omission/null/array at runtime; its Go type alone
		// cannot express that contract. Create requires a name; PATCH does not.
		body := components["InputProfileBody"].(map[string]any)
		body["properties"].(map[string]any)["library_ids"] = map[string]any{
			"anyOf":       []any{map[string]any{"type": "array", "items": map[string]any{"type": "integer", "format": "int64"}}, map[string]any{"type": "null"}},
			"description": "Omit to keep existing access (all libraries on creation); null means all libraries; [] means no libraries. Admin profiles always see every library.",
		}
		if r.handler == "createProfile" {
			create := map[string]any{}
			for key, value := range body {
				create[key] = value
			}
			create["required"] = []string{"name"}
			components["InputCreateProfile"] = create
			op["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"] = map[string]any{"$ref": "#/components/schemas/InputCreateProfile"}
		}
	case "downloadAsset":
		parameter("Range", "One byte range, open-ended or suffix; malformed, multiple or unsatisfiable ranges return JSON 416 with bytes */size. HEAD ignores Range.")
		parameter("If-Range", "Original sources have no strong validator; any If-Range returns full200. Replace partial bytes, never append that response.")
		media("application/octet-stream", "video/mp4", "video/x-matroska", "video/webm")
		full := responses["200"].(map[string]any)
		full["description"] = "Authenticated original bytes; private/no-store, exact Content-Length, no ETag or Last-Modified. HEAD has headers only. Revocation/drift after headers aborts the stream; verify final SHA-256."
		full["headers"] = map[string]any{"Content-Length": header("Representation byte count"), "Accept-Ranges": header("bytes"), "Content-Disposition": header("Safe attachment filename")}
		if r.method == "HEAD" {
			delete(full, "content")
		} else {
			responses["206"] = map[string]any{"description": "Single partial byte range", "content": full["content"], "headers": map[string]any{"Content-Range": header("bytes start-end/size")}}
			responses["416"] = map[string]any{"description": "Invalid/unsupported byte range; JSON error", "headers": map[string]any{"Content-Range": header("bytes */size")}}
		}
		responses["409"] = map[string]any{"description": "Preparation not ready or source changed"}
		responses["429"] = map[string]any{"description": "Transfer admission full; Retry-After:3"}
	case "getDownloadManifest":
		responses["409"] = map[string]any{"description": "Preparation not ready or source changed; release/reprepare if failed"}
	case "createDownload":
		schema := op["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
		props := schema["properties"].(map[string]any)
		props["mode"].(map[string]any)["enum"] = []string{"original"}
		props["file_id"].(map[string]any)["minimum"] = 1
		props["item_id"].(map[string]any)["minLength"] = 1
		props["item_id"].(map[string]any)["maxLength"] = 200
		props["idempotency_key"].(map[string]any)["minLength"] = 1
		props["idempotency_key"].(map[string]any)["maxLength"] = 80
		schema["additionalProperties"] = false
		op["x-arc-max-body-bytes"] = 4096
	case "listDownloads":
		op["parameters"] = []any{
			map[string]any{"name": "limit", "in": "query", "required": false, "schema": map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 50}},
			map[string]any{"name": "offset", "in": "query", "required": false, "schema": map[string]any{"type": "integer", "minimum": 0, "maximum": 10000, "default": 0}},
		}

	case "eventStream":
		parameter("Last-Event-ID", "Last received opaque SSE cursor (at most 80 characters). Retained process-local hints replay; invalid, stale or restart cursors require resync.")
		params := op["parameters"].([]any)
		params[len(params)-1].(map[string]any)["schema"].(map[string]any)["maxLength"] = 80
		components["EventHint"] = map[string]any{"type": "object", "required": []string{"topic"}, "properties": map[string]any{"topic": map[string]any{"type": "string", "description": "Invalidation topic; tolerate new topics and refetch state. Not a record patch."}, "library_id": map[string]any{"type": "integer", "format": "int64"}}}
		components["EventResync"] = map[string]any{"type": "object", "required": []string{"topic"}, "properties": map[string]any{"topic": map[string]any{"const": "resync", "type": "string"}}}
		op["x-arc-sse-events"] = map[string]any{"invalidate": map[string]any{"$ref": "#/components/schemas/EventHint"}, "resync": map[string]any{"$ref": "#/components/schemas/EventResync"}}
		responses["200"] = map[string]any{"description": "SSE hints, not guaranteed per-operation delivery. Refetch on resync; debounce invalidations. Heartbeats every 10s, reconnect after 30m; permissions rechecked during delivery. See docs/EVENTS-API.md.", "content": map[string]any{"text/event-stream": map[string]any{"schema": map[string]any{"type": "string"}, "example": "id: opaque-cursor\nevent: invalidate\ndata: {\"topic\":\"library\",\"library_id\":1}\n\n"}}, "headers": map[string]any{"Cache-Control": header("private, no-store"), "X-Accel-Buffering": header("no")}}
	case "subtitleVTT":
		media("text/vtt")
		responses["200"].(map[string]any)["description"] = "WebVTT (UTF-8), streamed while extraction runs. Extraction can fail after 200; no JSON error can then be sent."
	case "subtitleASS":
		media("text/x-ssa")
	case "transcodeFile":
		media("application/vnd.apple.mpegurl", "video/mp4")
		ranges()
		responses["206"].(map[string]any)["description"] = "Byte ranges apply to MP4 segments only; generated HLS playlists do not support ranges."
	case "stream", "streamExtra", "prerollVideo":
		media("video/mp4", "video/x-m4v", "video/x-matroska", "video/webm", "video/x-msvideo", "video/quicktime", "video/mp2t", "video/x-ms-wmv", "video/mpeg", "video/x-flv", "application/octet-stream")
		ranges()
	case "theme":
		media("audio/mpeg", "audio/mp4", "audio/flac", "application/octet-stream")
		ranges()
	case "trickplayBIF":
		media("application/octet-stream")
		ranges()
	case "trickplaySheet":
		media("image/jpeg")
		ranges()
	case "downloadDiagnostics":
		media("application/zip")
		responses["200"].(map[string]any)["headers"] = map[string]any{"Content-Disposition": header("attachment; filename=arc-diagnostics.zip"), "Cache-Control": header("no-store")}
	case "image", "collectionImage", "personImage", "seasonImage":
		media("image/jpeg", "image/png", "image/webp", "application/octet-stream")
		ranges()
		parameter("If-None-Match", "Cached artwork validator; matching cached images return 304. Remote provider fallbacks may redirect instead.")
		responses["302"] = map[string]any{"description": "Redirect to provider artwork when a cached local response is unavailable", "headers": map[string]any{"Location": header("Provider artwork URL")}}
		responses["200"].(map[string]any)["headers"] = map[string]any{"ETag": header("Validator for cached artwork, when available"), "Cache-Control": header("Matching v cache keys permit immutable caching; otherwise artwork revalidates")}
	case "artworkPalette":
		parameter("If-None-Match", "Previously returned palette ETag; unchanged palettes return 304.")
		responses["304"] = map[string]any{"description": "Palette unchanged; no body"}
		for _, status := range []string{"200", "304"} {
			responses[status].(map[string]any)["headers"] = map[string]any{"ETag": header("Palette validator"), "Cache-Control": header("private, no-cache"), "Vary": header("Authorization, Cookie")}
		}
	case "uploadArtwork", "uploadCollectionArtwork", "uploadSeasonArtwork":
		op["requestBody"] = map[string]any{"required": true, "description": "Raw JPEG, PNG or WebP bytes (not multipart); detected from the body. Maximum 20 MiB (20971520 bytes); 413 if larger, 415 for unsupported content.", "x-arc-max-bytes": 20971520, "content": content("image/jpeg", "image/png", "image/webp", "application/octet-stream")}
	}
	for status, value := range responses {
		resp := value.(map[string]any)
		headers, _ := resp["headers"].(map[string]any)
		if headers == nil {
			headers = map[string]any{}
			resp["headers"] = headers
		}
		headers["X-Request-ID"] = header("Server-issued decimal correlation ID; process-local, matches JSON error request_id and diagnostic logs. Not an authentication token.")
		if status == "200" && responses["206"] != nil {
			headers["Accept-Ranges"] = header("bytes for local file responses; not generated HLS playlists")
		}
		if status == "429" || status == "503" {
			headers["Retry-After"] = header("When supplied, wait this many seconds before retrying; SSE stream limits use 5 seconds.")
		}
	}
}

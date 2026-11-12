# #350: bound authenticated request bodies

The read-count regression reproduced protected JSON handlers consuming an entire 3 MiB string before rejecting it. Apply a 2 MiB read ceiling after authentication to mutating requests. Known oversized Content-Length is refused with 413 before decoding; streamed/unknown-length bodies are limited with MaxBytesReader and may receive the handler's existing 400 response. Endpoint-specific smaller limits remain in effect. Content-Type cannot exempt a JSON endpoint.

Raw artwork uploads keep their independent existing 20 MiB limit, identified by exact registered mux patterns for item, collection and season artwork. Both /api and /api/v1 route aliases are covered. GET streams/downloads are untouched; no WriteTimeout is introduced. Authentication remains before payload processing. This ceiling bounds consumed JSON bytes; handlers retain their existing first-value/EOF parsing semantics rather than pre-reading every request body.

This complements #355's unauthenticated limits and HTTP ReadTimeout/IdleTimeout. Preserve that PR's smaller unauthenticated limits and server timeouts when merging; this branch does not duplicate those independent changes. Existing 256 KiB prefs, 128 KiB queues and other tighter decoders remain tighter.

OpenAPI adds 413 on authenticated mutations and describes the server JSON ceiling while leaving the separate binary-upload description intact. Successful endpoints/response fields/types are unchanged. No frontend implementation, dependency or schema change. No production data or persistent server.

Tests count actual consumed request bytes for known/chunked bodies and spoofed content types across both aliases, prove unauthorized requests do not read the payload, and exercise a generated PNG larger than 2 MiB through the real upload route. Validation results are recorded after required checks finish.

No new logs, jobs or per-byte tracing. Existing request diagnostics handle ordinary 4xx outcomes; failures/cancellations do not add duplicate error records. Payload contents, secrets, raw URLs and SQL arguments are never logged by this guard.

Validation: read-count/status regression failed against unchanged main; all focused request/auth/upload controls passed. API-doc generation, Go vet, full Go suite and API/documentation race checks passed (API 299.585s; apidoc 9.050s). Semantic OpenAPI comparison found no removed responses and unchanged component schemas; the v1 contract file has no diff. No browser checks required for these server-only changes.

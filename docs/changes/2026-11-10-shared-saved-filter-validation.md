# Validate saved filters consistently

Smart playlists now reject unknown sorts, negative range values and ratings
outside 0–10 before creating anything. Home rows, smart collections and smart
playlists share saved-filter validation, including rejecting an inverted runtime
range. Open-ended ranges and syntactically valid future codecs remain supported.

Smart collections and playlists share the filter-to-query conversion, retaining
caller access, library kind, source-format filters and existing result limits.
Collection names use the existing collection-name limit, without accidentally
applying the shorter custom Home-row title limit.

Genuine red handler tests reproduced accepted invalid rules and persisted
playlists. Temporary-store checks cover rollback-before-create, equivalent
collection/playlist results and hidden-library access. Existing request validation
handles failures; no jobs, logging floods or raw filter logging were added.
Part of #350. API contracts and persisted filter shapes remain unchanged.

Validation: go run ./tools/apidoc, go vet ./..., go test ./... and
 go test -race -timeout 30m ./internal/api passed (293.751s race).
No generated API schema or v1 contract changes, frontend or dependency changes.

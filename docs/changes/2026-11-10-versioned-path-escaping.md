# Preserve escaped path segments in the versioned API

The /api/v1 alias previously discarded URL.RawPath, turning escaped slashes and
dot segments in item identifiers into routing separators or redirects. Preserve
the escaped suffix when rebasing the alias, and validate the namespace segments
individually so escaped namespace separators cannot become endpoints.

Regression tests first reproduced different routing/authentication between the
legacy and versioned paths. Tests cover encoded separators, dots, double escapes,
unreserved namespace escapes, query preservation and caller-request immutability.

Validation: go run ./tools/apidoc, go vet ./..., go test ./... and
 go test -race -timeout 30m ./internal/api passed (242.097s for race).
No API schema/contract, frontend, database or media changes. Existing request
logging handles these requests; no per-request logging or raw path output added.
Part of #350. Integration must preserve the other api.go middleware fixes.

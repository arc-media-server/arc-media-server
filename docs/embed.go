// Package docs embeds the generated OpenAPI document so the server can serve it at
// /api/openapi.json. Regenerate it with `go run ./tools/apidoc`.
package docs

import _ "embed"

//go:embed openapi.json
var OpenAPI []byte

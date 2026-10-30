# Server artwork palette API

Added profile-authenticated GET /api/v1/items/{id}/artwork/palette (with the normal /api alias),
returning four regional CSS hex colours, an opaque artwork version, selected image type/URL and
an explicit available=false fallback. Default backdrop can use primary only when no backdrop is
configured. Existing local/upload/provider precedence and item visibility guards are reused.
ETags cover the complete response; conditional requests still enforce permissions. A client
cannot supply arbitrary paths or URLs. The web/app renderer is unchanged.

Extraction and bounded memory/disk caches live in internal/palette. JPEG/PNG/WebP decode in Go;
GIF uses the first frame. Added x/image v0.47.0 for WebP; it requires x/sys v0.49.0 and x/text
v0.43.0, with the existing Go 1.27 toolchain retained. No host tool or FFmpeg runtime process was
added. Generation samples 4096 positions, weighting regional colours by alpha/chroma. Input,
pixel, concurrency and cache limits, plus ICC/EXIF and stat-fingerprint limitations, are in
ARTWORK-PALETTES.md. This is independent Arc behaviour, not a claim of Plex renderer parity.

Shared provider downloads now allow callers to stop waiting on cancellation while a bounded
background download fills the cache; active downloads are capped at four. Ordinary artwork
pruning skips the independently bounded derived palette directory. No new media scanning,
metadata provider, settings or owner-data access.

Logging: no success/poll/per-image info logs. Optional-decoration failures log at most once per
minute per server, with item ID/type/reason only; no raw error/URL/body/path/token. Cancelled
requests are not error logs. Tests verify warning bounds and privacy.

Validation: full go vet ./... and go test ./... passed (31 packages reporting pass), including
API/OpenAPI/v1-contract checks; focused palette/artwork/API race checks, production translation/app
TypeScript, e2e TypeScript and embedded build passed. Two existing artwork browser journeys passed.
Final API tests also verify that returned v1 image URLs work, identical bytes at a different
source invalidate ETags, and failure logs remain bounded/redacted. Tests use generated artwork, temporary stores and local provider fixtures. App visual
rendering, physical devices and owner deployment are separate acceptance work. Integration
examples and stable JSON fields are documented in docs/ARTWORK-PALETTES.md.

# Collection and playlist pages join the v1 contract

**What changed.** `GET /collections/{id}` and `GET /playlists/{id}` answered with untyped maps, so
the generated API description had no fields for them and `docs/api-v1-contract.txt` couldn't
promise any. They now answer with typed values (`collectionPage`, `playlistPage`) of exactly the
same JSON, and the contract lists their fields (`collection`, `items`, `playlist`, `entries` and
everything inside).

**Why.** The Android TV app is adding collection rows on title pages, a library's Collections
tab and playlist rows on Home (arc-android#1). Apps may rely only on what the v1 contract lists.

**Limits.** No behaviour change; the JSON is byte-for-byte the same shape.

**Checks.** `go vet` and `go test` for `internal/api` and `internal/apidoc`; `go run ./tools/apidoc`
regenerated `docs/openapi.json` and added the contract lines.

**Logging.** Nothing new: the handlers' existing failure handling is unchanged.

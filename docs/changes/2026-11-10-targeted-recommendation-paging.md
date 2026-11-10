# Evaluate only the recommendation row being continued

Continuing one row rebuilt all rows at 20 items, then all rows again at 500 for
a curated row. The measured 4,000-movie/80×50-episode fictional catalog allocated
about 27.8 MiB to return a 16 KB curated page.

The existing row builder now accepts an optional target identity. It skips
unrelated item queries and preserves the full view's descriptors, source/daily
identity, visibility and layout handling. Ordinary pages keep their filtered
query; curated pages retain the original 20-item visibility check and 500-item
candidate window. No global cache, provider call or new setting is introduced.

Five warm-cache cloud samples, with unchanged response sizes:
- Ordinary row: median 217.797 → 39.643 ms; 1.982 → 0.342 MiB allocated.
- Curated row: median 482.731 → 44.945 ms; 27.843 → 3.625 MiB allocated.
These fixture observations are not an Unraid/NAS or general latency guarantee.
Home, playlist and collection paths remain controls; playlist optimization is
separate.

The opt-in allocation regression first failed at 27.845 MiB (>16 MiB generous
budget) and passed at 3.625 MiB. Permanent controls compare every targeted row's
public JSON with its full-view counterpart for movies/shows and existing
director/genre/age fixtures. Existing continuation, hidden/stale identity, paging
and current-access tests pass.

Validation: go run ./tools/apidoc, go vet ./..., go test ./... and full API race
passed (250.724s). Additional test-only row-equivalence controls passed in a full
Go run and focused race check. No generated API/v1 contract, schema, frontend,
dependency, media or logging changes. Part of #350 code-quality hot-path review.

Shared opt-in measurement helpers overlap the playlist-summary PR: retain the
helper once and both allocation tests on integration.

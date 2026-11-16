# Reduce repeated work across Recommended rows

The owner's Series Recommended request spent 2.961 of 2.965 seconds constructing 15 rows,
even after the earlier watch-history query fix. Recently Watched was the largest step at
735 ms, with smaller per-row costs accumulating sequentially.

- All ordinary recommendation families fetch one extra card to determine continuation,
  without a full matching-title COUNT. Browsing and row-continuation responses keep exact totals.
- Recently Watched movies/episodes start from the profile's indexed watch history rather
  than sorting the whole catalogue. Keep the existing title ties, presence, library/age/label
  restrictions and fallback ordering for other kinds/sorts.
- Repeated cards reuse watch state and series unwatched counts within the request. Episode
  parents, display ordering and season artwork are shared across episode rows; Up Next shares
  its ordering context with Continue Watching and Recently Watched. Nothing persists between
  requests or profiles, so new requests see progress and settings changes.
- Read layout before generating rows. Skip hidden families and unnecessary person/genre/
  similarity prerequisite lookups. Keep favourite-genre selection stable when only its Top row
  is visible. All visible rows, IDs, styles, permissions and continuation URLs remain available.
- Ordinary row debug timings now include separate `_query` and `_details` values alongside
  the existing row totals. One bounded debug record per full request; preparation is guarded
  by the debug level, with no new info-level logging, raw URLs, SQL or credentials.

No schema, dependency or API response changes. The endpoint still returns the complete visible
page. Kodi's separate 12-row display cap is not applied globally to web or other clients.

Regression coverage includes watched-date ties, missing files, parent age restrictions, library
access, window boundaries, exact continuation totals, request/profile cache isolation, and
proof that cached cards and hidden families do not issue their discarded database work.

Validation (Go 1.27.0, prepared Jellyfin FFmpeg 8.1.3, temporary stores/local fixtures):
- `go vet ./...`, full `go test ./...` (37 packages passed; 7 without runnable tests), production
  web/i18n/unit/app TypeScript build, e2e TypeScript, embedded-server build, API documentation/
  v1 contract checks and whitespace checks passed. Generated API documents are unchanged.
- Affected API/store race checks passed. The existing 20-second writer-queue test initially
  timed out during concurrent verification; the same store selection passed alone (30.021s),
  without changing its setup, assertions or timeout. Initial sandbox/overlapping asset-build
  attempts were superseded by final permitted runs after generated assets were stable.
- Pinned Chromium: 3/3 Recommended journeys passed (1.7m), covering real movie/series rows,
  empty states, links/titles and staged/persisted/reset shared row settings. No full browser
  suite or production/device acceptance claimed.

Three-run synthetic medians against parent 7819257: 276 shows ×30 episodes, 178→138 ms;
2,000 movies, 187→173 ms. A separate fixture with episode orders and descriptions changed
211→152 ms (14 rows). In the TV fixture Recently Watched changed from about 36 to 4 ms.
Samples vary and are local measurements; the owner's post-deployment timings remain necessary.
No owner database/settings/media, production keys or Unraid were accessed. No tools installed.

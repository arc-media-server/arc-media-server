# Avoid full playlist contents just to list summaries

The review's hot-path concern was measured in a disposable indexed catalog:
4,000 movies, 80 shows with 50 episodes each, 20 smart and 20 manual 500-entry
playlists, and 40 collections. No media files or providers were read.

Listing playlists previously scanned every full item to return counts, duration
and four images. Aggregate visible manual-entry counts/durations in one grouped
query, select only four distinct manual poster identities, and use a narrow
bounded projection for movie smart summaries. Reuse the exact item ordering logic;
keep full detail/playback pages and smart-show expansion unchanged. Preserve the
500-item window, duplicate entries, duration, artwork locks and library access.

Five warm-cache samples in this cloud fixture:
- Before: median 354.465 ms, 66.819 MiB allocated, 5,906 response bytes.
- After: median 115.244 ms, 19.493 MiB allocated, the same response size.
These are fixture observations, not an Unraid/NAS or general latency guarantee.
Home/collection-grid allocations stayed about 3.33/6.36 MiB; their bounded timings
were much smaller than the two hotspots, so no speculative caching/index changes
were added there. Recommendation paging is handled separately.

An opt-in regression first failed at 66.841 MiB against a generous 32 MiB budget
for these 40 summaries; final allocation check passed at 19.483 MiB. The test
compares the 500-item counts/durations against full playlist pages. Additional
tests preserve poster order, artless titles, duplicate episode-parent posters,
hidden libraries and empty access. Equivalence checks caught a missing artwork
lock field in the initial projection; the focused failure and correction are
retained before final verification.

Validation: go run ./tools/apidoc, go vet ./..., go test ./... passed. Final API
race passed (254.517s); store race command passed from cache (the prior full store
race was 231.823s). No generated API/v1 contract, schema, frontend or dependency
change. Default checks skip the opt-in fixture; run with
ARC_RUN_REVIEW_HOT_PATHS=1 go test ./internal/api -run 'TestReviewHotPathMeasurements|TestPlaylistListingAllocationBudget' -count=1 -v

Shared measurement-helper changes are also used by the recommendation-paging PR;
retain the helper once and both allocation tests on integration. Preserve #406's
saved-filter validation/query conversion in playlists.go. No new app logging or
raw SQL arguments/keys/URLs. Part of #350 code-quality hot-path review.

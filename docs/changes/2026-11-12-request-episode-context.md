# Request-local episode context

CQR06 shares parent-series and display-position resolution across Home, recently added, playlists and episode search. Each request reads a parent and its ordering once, caching missing parents too; search no longer rebuilds the same order for every episode. No cross-request/profile cache. Endpoint-specific poster selection, grouping, scoring and watch-state filling remain local.

Database failures propagate instead of search/playlists silently treating them as absent metadata. Missing parents remain a valid empty context, with each endpoint retaining its existing presentation choice. The helper does not replace access checks or resolve additional media.

Go vet, full Go tests with serial packages, focused API search/recent/playlist/Home race checks, and the final parent/order cache race regression pass. The cache test verifies actual display positions, reuse after the database becomes unavailable, cached missing rows and a new request's database error. API documentation regeneration produces no diff. Temporary fixtures only; no measured latency or owner-server acceptance claim.

Logging: actionable failures continue through the existing request boundary once. No success/poll/per-episode logs, secret data or new background work.

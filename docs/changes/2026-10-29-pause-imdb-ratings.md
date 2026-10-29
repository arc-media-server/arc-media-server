# Pause IMDb rating ingestion and web presentation

Owner requested temporary removal after diagnostics showed multi-minute writes. The ratings
manager now runs only optional OMDb critics updates; it does not download or reapply IMDb data.
The web hides IMDb badges/votes, sort choices and the weekly-update settings row. Existing IMDb
IDs and stored/API data remain. Metadata-provider scores and personal ratings remain available.
This does not prove or fix the independently observed library-list hang.

`docs/RATING-SOURCES.md` records the deferred code-removal decision, compatibility boundaries and
an official-source comparison of TMDB, OMDb, licensed IMDb and personal ratings. No new provider,
key, subscription, data deletion or bulk migration is introduced.

Validation (prepared cloud tools, isolated stores, generated clips and local provider fixtures):
- `go test ./internal/ratings -count=1`: passed, including missing/cached dataset cases that
  assert no IMDb request/application while optional critic scores still update.
- `CGO_ENABLED=1 go test -race ./internal/ratings -count=1`: passed.
- `go vet ./internal/ratings ./cmd/ams`: passed.
- `go test ./internal/store ./internal/api -timeout 5m`: both packages passed.
- UI production build (including 20/20 translation checks), app TypeScript and e2e TypeScript:
  passed. Existing Vite configuration/dynamic-import warnings remain.
- Pinned Chromium: 3/3 focused journeys passed, no retries (new stale-IMDb/provider/critic/personal
  rating visibility and saved-sort fallback; existing watchlist/rating/refresh recovery; existing
  profile-specific sort direction). Full browser/Go/race/container suites were not run.
- Disposable embedded-server build and `git diff --check`: passed.

The first new browser test used the wrong existing accessible label (`Sort: Title` rather than
`Sort by Title`); corrected the test and reran successfully. A sandbox blocked local fixture
sockets; successful tests ran with approved execution permissions. One overlapping vet/build
attempt encountered assets being replaced by the UI build; sequential rerun passed. No tools
or dependencies were installed, and no production or physical-device checks were performed.

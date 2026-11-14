# Aggregate episode history before joining favourite genres

FavouriteGenres previously joined every watched episode to its parent show's genre rows before
grouping. For a long series this repeatedly joins the same small set of genres thousands of
times. First aggregate watched records by title, retaining their count, then join each title's
genres and sum the weights. Multiple watched episodes continue to carry multiple votes; this
is not a distinct-show count. Preserve profile/library scope, genre-name ties, limits, direct
series state and played-only semantics (in-progress alone is not a vote).

No persistent cache, settings, migration or API changes. A focused equivalence test compares
against the previous query for episode/movie/direct-series history, multiple genres, profile
and library isolation, empty results and different limits. An opt-in 100,000-episode query
measurement checks both variants return the same genre ordering:

`ARC_RUN_GENRE_SCALE=1 go test ./internal/store -run '^TestFavouriteGenreScaleMeasurements$' -count=1 -v`

Existing Recommended favourite_genres timing and request/error logging covers this synchronous
query. No additional logging, per-item records or expensive disabled-log preparation is added.
All tests use disposable synthetic data, without owner media/settings/keys or Unraid.

Same-store query comparison (100,000 episodes; 75,000 played records and multiple genres):
three-repeat median 165.8→119.8 ms with identical genre ordering. In the complete Recommended
response, FavouriteGenres changed 141→111 ms (1,000×100) and 136→108 ms (200×500); whole-page
medians were 1,243→1,218 ms and 1,326→1,324 ms respectively. All full JSON snapshots matched
for both Series and Movies. The isolated stage improvement does not imply a large whole-page
speedup; other stages and sample variance dominate the remaining time. Fixed history timestamps,
same UTC day, first plus five full-page repeats, no concurrent measurement/build jobs.

Validation: `go vet ./...`, full `go test ./...` (37 packages passed; 7 without runnable tests),
focused store race checks (7.972s), weighted-history equivalence and opt-in scale comparisons
passed. `git diff --check` passed. Web unchanged; no browser or owner-server acceptance claimed.

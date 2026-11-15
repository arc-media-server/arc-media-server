# Fewer Recommended rows by default

A Recommended page built up to 15 rows. On the owner's server the Series page took about 1.9 s
with every row costing 40–280 ms, so the remaining time was spread across the rows rather than
one slow query. The owner chose fewer rows by default (14 November).

- A library that hasn't been customised in Edit Recommended now starts without Rediscover,
  Top in Genre, the featured actor/director row and Top Unwatched. They overlap other rows
  (Top Unwatched with Top Rated and Start Watching, Top in Genre with More in Genre) or are
  extras, and each is a slow query on a large library. Hidden rows aren't built, so they cost
  nothing. Edit Recommended still lists them and turns them back on.
- Because You Watched shows two rows (was three) and More in Genre one (was two).
- A layout saved in Edit Recommended is unchanged: the editor saves every family, so these
  defaults only apply to libraries never edited and to families added later. The layout API
  now returns every family with its effective visibility, and Reset goes back to these
  defaults.

On the owner's Series log this removes about 0.8 s (about 1.9 s to about 1.1 s) and about six
rows. Logged on #80 as a default to revisit.

Validation: `go vet ./...`, `go test ./internal/api ./internal/store`, `npx tsc -b`, the
production web build, `go run ./tools/apidoc` and `git diff --check`. The Recommended test checks
the default rows and that the hidden families come back when turned on. No new logging: the
existing Recommended debug timings simply list fewer steps.

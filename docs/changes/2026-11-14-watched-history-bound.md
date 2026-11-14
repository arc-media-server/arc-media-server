# Bound recent-history work for sparse watched libraries

The history-first window from #465 could walk 100,000 unrelated episode records before finding
an older movie in a small library. Replacing the forced join with a planner-selected join fixed
that case but lost the dense episode speedup (about 1 ms became 422–489 ms in the focused check).

Use an indexed timestamp horizon from the first 512 profile records. Probe only newer records;
accept the result only when it fills the requested window. Otherwise use the catalogue-driven
query with every original filter and exact ordering. Exclude the entire cutoff timestamp tie
from the probe so titles tied at the boundary are always sorted completely by the fallback.
NULL timestamps, no matches, older-only libraries, restricted filters and large offsets fall
back without truncating results. Exact browsing counts and continuation behavior are unchanged.
No schema, dependency or API changes. The 512-record budget is internal, not a setting.

Regression tests exercise the catalogue fallback plan, sparse/dense libraries, timestamp ties,
NULL dates, denied libraries, empty filters and pagination. The fallback-plan check failed
against main before the fix. An opt-in mixed-library measurement fixture keeps the scale case
reproducible without making ordinary tests create 100,000 episodes:

`ARC_RUN_WATCHED_SCALE=1 go test ./internal/store -run '^TestWatchedWindowScaleMeasurements$' -count=1 -v`

Same-store measurements with 100 movies behind 100,000 newer watched episodes, first plus three
repeats: previous forced-history movie median 140.9 ms, bounded version 3.35 ms; catalogue-only
1.48 ms. Dense episode bounded median 0.95 ms, previous history 0.86 ms, catalogue roughly
777 ms in this run. Every compared 21-item window was deeply equal. These are local query
measurements, not a guarantee for a deployed server or a complete page timing.

Existing query/detail/request debug records and error propagation remain sufficient; no new
per-item or info logging. Underfilled probes are normal selection decisions, not warnings.
Only temporary stores and synthetic data were used; no owner media/settings/keys or Unraid.

Validation: `go vet ./...`, full `go test ./...` (37 packages passed; 7 with no runnable
tests), full `go test -race ./internal/store` (600.071s), focused window tests and the opt-in
scale comparison passed. `git diff --check` passed. Web source is unchanged; no browser suite
or production/device acceptance is claimed.

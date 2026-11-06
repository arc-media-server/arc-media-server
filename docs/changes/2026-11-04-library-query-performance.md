# Measured library-query improvements and contention investigation

Owner requested backend investigation of slow watchlist writes/disappearing libraries and realistic
large-library performance. First on `codex/library-query-performance` (stacked on the PostgreSQL work);
moved onto main on 6 November once PostgreSQL was parked (#308). No owner database/media/server accessed.

Existing read/write separation, queued writers and asynchronous session timestamps held up in the
fixtures: libraries/Home stayed available during scan-style touches, metadata/background-job updates
and authenticated watchlist mutations. Cancelled queued mutations never applied later. The original
minutes-long stall/disappearance was not reproduced; owner deployment acceptance remains open.

Measured changes:

- Count libraries without fetching/sorting unused item pages or listing libraries twice. Reuse the
  already loaded permitted libraries, filtering denied libraries before count work. Count semantics
  remain the same; no cache/stale counts added.
- Replace all-episode window ranking for recently added series with indexed per-parent newest-date
  selection, full tie-breaking, then complete visible-group expansion. Use existing indexes only.
  Preserve offline-file, library/age/label access, missing-arrival and ordering/group-count behavior.

Five-call paired API samples with 10,000 movies, 1,000 shows and 100,000 episodes measured recent
series groups at 827.9 → 136.9 ms median (about sixfold). A burst-import/tied-date core query measured
733.2 → 422.4 ms. Other small changes are sampling noise, not claimed gains. Library listing measured
29.5 → 25.8 ms; Continue Watching/Up Next about 109 ms and one series' seasons about 5 ms. Twenty
concurrent watchlist requests all saved while library/Home/browse reads stayed consistent; p95 was
1.56/3.83 ms before/after, so no watchlist speed improvement is claimed.

Reproducible opt-in fixture harnesses, plans, samples, methodology and limits:
[LIBRARY-PERFORMANCE.md](../LIBRARY-PERFORMANCE.md). No timing thresholds are CI gates. Previous
window SQL remains a test oracle alongside manual complete-group assertions and access/offline/tie
cases. PostgreSQL retains its existing negative-LIMIT error; SQLite retains unlimited-negative behavior.

Actual validation:

- Full Go: 35 packages passed; 591 test/subtest passes, zero failures; 38 skipped tests (PostgreSQL
  integrations and two opt-in measurements), eight packages without runnable tests.
- PostgreSQL 17.6 shared store: 139 passes, one opt-in measurement skip, zero failures. No schema
  or migration changes; PostgreSQL 16/18 performance was not measured in this task.
- Focused final race: eight passing results across store/API, plus the existing 11-second writer
  queue test passed separately under race. The first overlapping race run exceeded that test's
  existing 20-second setup/test deadline under concurrent suite/build load; raw failure retained.
- Go vet, production web/i18n/app TypeScript build, E2E TypeScript, generated API/v1 contract and
  whitespace checks passed. No generated API contract changes.
- Pinned Chromium 4/4 affected journeys passed, zero skips/retries, with generated clips/local
  provider fixtures and embedded-server build: unchanged series scans, alternate episode order,
  profile/library permissions and Recommended rows.

No new runtime logging: existing slow/unfinished request stages and pool counters remain, with no
SQL/path/token logging or per-query info flood. No new dependencies/tools installed. Temporary
PostgreSQL and browser servers stopped. No full browser/container, actual owner deployment, real media,
network/provider latency, sustained load or cold-storage acceptance. Original owner stall needs
confirmation during normal use after deployment; capture fresh diagnostics while stalled if it returns.
Raw evidence: `/workspace/arc-cloud-evidence/library-performance/REPORT.md`. 

## Moved onto main (6 November)

PostgreSQL work is parked (#308), so this change was taken off the PostgreSQL branches and applied
to main on its own. It applied without conflicts. On main, `go vet ./...` and `go test ./...`
pass, with SQLite only. The PostgreSQL measurements above came from the earlier branch.

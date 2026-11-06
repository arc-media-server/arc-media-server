# Library query performance and contention

This investigation uses generated database rows and temporary stores, never owner databases or
media. SQLite is the performance target (PostgreSQL support is parked, #308). Results are local measurements, not production
latency promises or proof that the owner's original stall has been resolved.

## Findings and changes

The existing split read/write pools, serialized writer queue and asynchronous session-activity
updates already kept reads available during the tested write contention. Concurrent authenticated
watchlist writes, scan-style file touches and metadata updates completed without lost changes or
empty library responses. A deliberately held writer did not block committed library/browse reads;
a cancelled queued watchlist write did not apply later. No new lock, timeout or cache policy was
justified by these observations. The owner's minutes-long stall/disappearance was not reproduced.

Two unnecessary query costs were confirmed:

- Library listing obtained counts through `ListItems`, fetching and sorting a sample item page for
  each library and loading the libraries twice. `LibraryCountsFor` now runs count-only queries on the
  libraries already loaded and allowed by the caller. Existing count semantics remain unchanged;
  denied libraries are filtered before their count work. No count cache or stale data is introduced.
- Recently added series ranked every eligible episode with a window sort before selecting show
  groups. The new query finds eligible parents, seeks each parent's newest visible available date
  through the existing `items_parent_added` index, breaks ties among that date's episodes, selects
  groups, then expands their complete visible episodes. It retains arrival sequence, release-date
  and ID ordering, hidden/age/library restrictions, offline-file handling and full group counts.
  SQLite's selected-group cross join keeps expansion bounded to selected parents. No new indexes,
  database migrations, API payloads, configuration options or dependencies are needed.

Other sampled browsing, facet, Continue Watching/Up Next and series-ordering paths did not show a
comparable bottleneck. They were measured rather than changed speculatively. This is not a claim
that every Home row, filter combination or production library size has been exhaustively profiled.

## Measurements

Five requests per endpoint on the same 10,000-movie / 1,000-show / 100,000-episode fixture. API calls
traverse the real versioned router, Bearer/profile checks, store reads and JSON serialization through
httptest; they exclude network transport, browser rendering and owner disk/network performance.
Both versions use the same fixture and prepared compiler. The baseline is unchanged commit 0ca1d9b
with only untracked measurement helpers added in a separate detached worktree.

| API work | Baseline median | Candidate median |
| --- | ---: | ---: |
| Libraries and counts | 29.5 ms | 25.8 ms |
| Movies, recently added | 77.1 ms | 64.0 ms |
| Movies, genre + unwatched filter | 35.2 ms | 30.6 ms |
| Recently added series groups | **827.9 ms** | **136.9 ms** |
| Continue Watching including Up Next | 109.3 ms | 109.0 ms |
| One series' seasons/order | 5.0 ms | 5.0 ms |

The approximately sixfold series-row improvement is the substantive measured change. Small
unmodified-endpoint differences are normal run-to-run noise, not claimed optimizations. In a
separate burst-import fixture (all episodes of a show share an added timestamp), the same core
recent-group query measured 733.2 → 422.4 ms; tied-date work reduces the benefit as expected.

The 100,000-episode concurrent API fixture completed all 20 watchlist changes while reading library,
Home and browse endpoints. Baseline/candidate watchlist p95 were 1.56/3.83 ms, with maxima 3.65/4.49 ms.
Both remained responsive; this does not establish a watchlist speed improvement. The cumulative
writer-pool counters showed queued work (43/44 waits, 10/8 ms), with no mutation failures or lost state.
Synthetic workload duration was short; sustained scans, external database processes, real provider
latency, filesystem stalls and the original owner deployment remain outside this evidence.

The data has fake names/paths, two media streams per present file, first-arrival rows, profile watch
state and episode orderings. Metadata text/artwork/credits are not as large as a real enriched catalog.
Runs used ANALYZE, ordinary process/filesystem caches and five samples without OS cache eviction.
These are not cold-storage benchmarks. Raw samples/plans/corrections are preserved outside Git in
`/workspace/arc-cloud-evidence/library-performance/`.

## Reproduce safely

In the prepared cloud environment, activate `/workspace/arc-cloud-tools/activate.sh` in each shell.
All following tests create their own temporary databases; they accept no owner database path.

```sh
# Store-level samples at 10,000 and 100,000 episodes, plus 10,000 movies.
ARC_RUN_LIBRARY_BENCH=1 go test ./internal/store -run '^TestLargeLibraryMeasurements$' -count=1 -v
# Recent groups after batch imports, including the actual SQLite query plan.
ARC_RUN_LIBRARY_BENCH=1 ARC_BENCH_BURST=1 ARC_BENCH_QUERY=recent_episodes ARC_BENCH_EXPLAIN=1 \
  go test ./internal/store -run '^TestLargeLibraryMeasurements$' -count=1 -v
# Actual API responses and a large concurrent mutation/read fixture.
ARC_RUN_LIBRARY_BENCH=1 go test ./internal/api \
  -run '^TestLargeLibraryAPIMeasurements$|^TestAuthenticatedLibraryRequestsDuringCatalogWrites$' -count=1 -v
```

Normal tests use small bounded fixtures and opt-in benchmarks skip by default. No timing threshold
is a CI gate. Test correctness is asserted separately: the previous window query is retained only
as a test oracle, with manual complete-group assertions and permission/missing-file/tie cases. Real
PostgreSQL additionally checks its existing negative-LIMIT rejection rather than imposing SQLite's
unlimited-negative behavior on it.

## Logging and owner follow-up

No new runtime logging or per-query tracing. Existing slow/unfinished-request diagnostics, request
stages and database-pool counters remain the diagnostic boundary, without info-level hot-loop logs.
No raw SQL, media paths, tokens or provider keys are added to logs. Query plans and synthetic timing
records are emitted only by explicitly invoked developer tests.

After deployment, ordinary owner use must confirm whether the earlier library/watchlist symptom
returns. If it does, capture a fresh diagnostics bundle while it is stalled, before restarting,
including the approximate time and active scan/maintenance operation. Cloud fixtures alone cannot
close that owner-server acceptance item.

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

## 14 November: enriched Recommended and facet follow-up

The earlier fixture above did not represent the owner's full watch history, cast data or
long-running shows. New server-side observations after #465 put Series Recommended at 2.180s
and Anime at 1.280s, while Recently Watched itself fell from the earlier 735ms to 6ms. Those
are separate requests/libraries, not a controlled whole-page comparison. The same server
snippet includes a 1.823s facets request. Kodi rendering/artwork is outside these server times.

Astra's focused source/profile investigation found SQLite execution dominating the enriched
fixture (~87% of sampled CPU). Continue/up-next, person/similarity queries, history aggregates
and unwatched counts remain meaningful costs; JSON serialization was comparatively small.
The older random 10–25s outliers did not recur in the instrumented comparison. Their cause
remains unproven: cumulative throttling counters cannot attribute historical stalls to CPU
quota, database locks, disk or GC.

Four narrow follow-ups were developed on separate branches from main a799187:

| Scope | What changes | Local result |
| --- | --- | --- |
| [Sparse watched library (#466)](https://github.com/arc-media-server/arc-media-server/pull/466) | Bound recent-history probes, with complete catalogue fallback | A 100-movie window behind 100k newer watched episodes: 140.9→3.35ms; dense episodes remain ~1ms |
| [Continue/up-next (#467)](https://github.com/arc-media-server/arc-media-server/pull/467) | Query ordered successor candidates in 128-entry windows; avoid unused special-placement maps | 200×500 Continue stage 491→388ms; Series allocation 55.8→21.5MB/request |
| [Favourite genres (#468)](https://github.com/arc-media-server/arc-media-server/pull/468) | Aggregate episode votes per title before joining its genres | Isolated query median 165.8→119.8ms, preserving per-episode weighting |
| Library facets | Share accessible present streams across four media aggregates | 1,000×100 Series facets 2,057→1,254ms; Movies 1,183→569ms |

These are separate-query/branch measurements, not additive savings or a combined deployment
claim. Continue's 1,000×100 stage was essentially unchanged (283→285ms); its full Series page
was 1,243→1,257ms. For 200×500 the page changed 1,326→1,205ms. FavouriteGenres improved its
stage (141→111ms and 136→108ms) but barely moved complete pages (1,243→1,218ms and
1,326→1,324ms). Other work and sample variation still dominate. Movie page differences on
these branches are not claimed improvements.

Full Recommended comparisons used 10,000 movies plus 100,000 episodes, roughly 68k watch-state
records, shared cast/directors, genres, descriptions and real display ordering entries. Both
1,000×100 and 200×500 shapes were tested, with first plus five repeated requests per library.
History timestamps and UTC-day selections were fixed/matched; whole decoded JSON snapshots
matched baseline in all four cases. Both facet payload hashes matched on every first/repeated
request (first plus three repeats per library). Fixture setup ran before measurement; benchmark
processes ran sequentially without concurrent builds/tests. These are in-process HTTP responses
including authentication and encoding, excluding network/Kodi/browser/storage acceptance and
without dropping OS caches. A few milliseconds of cgroup throttling occurred in the final
Continue run, not enough evidence to explain the historical outliers.

A simpler planner-selected watched JOIN fixed the sparse library but cost 422–489ms on the
dense episode window, so it was rejected. Continue's initial 32-entry probe added round trips
for shorter shows; the final 128-entry probe retained their stage latency. Initial facet samples
before excluding unused subtitle streams were also retained. No preliminary/outlier samples
are substituted for final-implementation results. Raw logs, fixed-fixture measurement helper,
CPU profiles, snapshots and the investigation report are saved outside Git under
`/workspace/arc-cloud-evidence/recommended-pipeline/` and
`/workspace/arc-cloud-evidence/recommended-followups/`.

Remaining work is deliberately explicit:

- FeaturedPeople (~201ms in the owner's sample), three Similar sources and the recently-watched
  title/history queries remain. Evaluate shared request-local aggregation or targeted query-plan
  changes with exact profile/access/tie and episode-vote semantics before implementing them.
- Continue still builds the selected display orders and selects recent series. A bounded
  candidate window reduces state/file work, but does not make the entire request constant time.
- Large simultaneous scans/maintenance, rare filters, multiple profiles and real disk/network
  behavior need separate acceptance. The old library-disappearance/watchlist stall remains an
  owner-server validation item; these fixtures do not close it.
- Correlate the next deployment's per-request row/query/details timings. If a long stall recurs,
  capture diagnostics before restart; compare the same library/request scope rather than
  subtracting unrelated server and client timings.

The new opt-in tests accept no external database/media path and skip in ordinary suites:

```sh
ARC_RUN_WATCHED_SCALE=1 go test ./internal/store -run '^TestWatchedWindowScaleMeasurements$' -count=1 -v
ARC_RUN_GENRE_SCALE=1 go test ./internal/store -run '^TestFavouriteGenreScaleMeasurements$' -count=1 -v
ARC_RUN_FACETS_SCALE=1 go test ./internal/api -run '^TestLibraryFacetScaleMeasurements$' -count=1 -v
```

Run each command in the branch/merged tree containing that test. Existing full Recommended
measurements remain available through `ARC_RUN_RECOMMENDED_SCALE=1`. No persistent cache,
new settings, schema/index migration or public API changes were introduced by these follow-ups.

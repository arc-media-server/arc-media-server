# Speed up collection lookup on item overviews

Opening a movie or editing from its poster used to count every collection in
the movie's library while loading the item details. Arc now counts only
collections that could appear on that item's page. Collection minimums, profile
visibility, and suppression of same-name imported duplicates still use the
shared collection rules.

The same local 10,000-movie fixture was measured before and after. Each value
is the median of five item-details requests. The final timing run had no other
validation jobs running.

| Automatic collections | SQLite before | SQLite after | PostgreSQL 19 before | PostgreSQL 19 after |
| ---: | ---: | ---: | ---: | ---: |
| 0 | 0.6 ms | 1.5 ms | 5.7 ms | 5.5 ms |
| 300 | 518 ms | 2.9 ms | 862 ms | 8.8 ms |
| 1,200 | 2.21 s | 2.4 ms | 3.12 s | 6.0 ms |

These measure the API handler on a synthetic catalogue. Browser rendering,
network latency to the owner's server and live metadata providers were outside
the benchmark.

Vet, the SQLite Go suite, the web typecheck and Vite build, the embedded build,
and the pinned Chromium artwork-edit journey passed. Focused PostgreSQL 19
race tests passed for collection membership, access and deduplication. No
migration, API contract or additional logging is needed.

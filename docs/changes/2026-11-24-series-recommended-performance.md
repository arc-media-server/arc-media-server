# Series Recommended Recently Added query (#524)

Recently Added now seeks to each show's latest visible episode date using the existing episode-date
index. For the sequence tie-break it checks only episodes at the latest date with a recorded arrival.
Missing files, missing arrival records, visibility and newest/oldest pagination follow the same
rules as before. There are no schema or API changes.

Median server times across five calls on a disposable catalogue of 10,000 movies, 500 series,
20,000 episodes and 600 collections, with #527 applied in both runs:

| Measurement | SQLite before → after | PostgreSQL 19 before → after |
| --- | ---: | ---: |
| Recently Added query | 10 → 9 ms | 71 → 37 ms |
| Full Series Recommended request | 72 → 75 ms | 274 → 263 ms |

The historical full-request medians before #527 were 80 ms on SQLite and 379 ms on PostgreSQL.
The row timings are more stable than the small change in total response time. The measurements
exclude network, client rendering and owner data. Existing debug logs still time each row.

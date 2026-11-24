# Search collection performance (#524)

Search checks collection names before counting visible titles. Matching collections still use the
library's visibility rules, including the automatic collection minimum and suppression of imported
duplicates. Search uses the same title scoring and returns the same ordering and artwork.

Five authenticated API calls per case on each disposable database. The fixture contains 10,000
movies, 500 shows, 20,000 episodes and 600 automatic collections. Times are median server-side
latency. The baseline is main at 4abc823.

| Query | SQLite before → after | PostgreSQL 19 before → after |
| --- | ---: | ---: |
| Movie 100 | 1,515 → 88 ms | 2,083 → 158 ms |
| Collection 100 | 1,554 → 88 ms | 1,945 → 154 ms |

The regression test covers accent-insensitive collection matching and a profile whose excluded
label drops an automatic collection below the minimum. These measurements exclude client and
network time. Existing request logging covers this path.

With the separate full-collections change (#525) applied first, the two cases measured 192 and
158 ms on SQLite and 203 and 231 ms on PostgreSQL. With both changes, they measured 92 and 90 ms
on SQLite and 139 and 146 ms on PostgreSQL.

Go vet, the embedded build, TypeScript, the production web build and all three Search browser
journeys passed. The full Go suite reported one failure in the lifecycle shutdown diagnostic
test, which passed on an isolated rerun. The Search regression also passed on PostgreSQL.

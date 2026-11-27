# Movie filter facets (#524)

Movie resolution counts now use the same present-file and stream lookup as codec, audio channel
and dynamic range counts. The query visits child episodes only for series and retains access
checks for those episodes. Each title still counts once per value, and resolution buckets
keep their highest-first ordering. The API and schema are unchanged.

Median server response across five requests per engine on a disposable fixture with 10,000
movies, 500 series and 20,000 episodes, using main at 4abc823 as the baseline:

| Facets | SQLite before → after | PostgreSQL 19 before → after |
| --- | ---: | ---: |
| Movies | 275 → 133 ms | 231 → 184 ms |
| Series | 291 → 200 ms | 230 → 229 ms |

Before the change, movie stream aggregation took a median 190 ms on SQLite and 136 ms on
PostgreSQL. The separate resolution query took 38 ms and 40 ms, and JSON writing took less
than 0.1 ms on both. Response bytes matched before and after across both engines. Fixture
times exclude browser rendering, network transfer and owner data.

The store regression covers duplicate video tracks, widescreen and missing dimensions,
multiple resolution values, offline files and age-restricted titles. Request timing and
existing error logging cover this path.

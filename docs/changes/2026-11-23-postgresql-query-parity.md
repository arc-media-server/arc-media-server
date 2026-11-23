# PostgreSQL query parity for scanning and browsing

[#308](https://github.com/arc-media-server/arc-media-server/issues/308)

Move the matching conditions from `CROSS JOIN … ON` into `WHERE` clauses. PostgreSQL
rejects the former syntax. Keeping the cross joins preserves SQLite's intended join
order for watch history and recommendation queries. Ratings with missing critics
scores now sort last on either engine.

Reuse the existing SQLite scenarios in the PostgreSQL release gate. Each run gets its
own database inside the disposable fixture. SQLite query-plan assertions still run
with SQLite and PostgreSQL checks the same result assertions.

| Workflow | Fixture coverage |
| --- | --- |
| Library scanning | Targeted movie and TV scans, multi-episode links, local artwork, file renames and watch-state preservation |
| Background work | Analysis backlog, Activity labels, Run Now boundaries and completed-job bookkeeping |
| Recommendations and watch progress | Profile and library boundaries, recent watches, resume state, episode counts, credited people and paging |
| Browsing and filters | Labels, resolution and stream facets, age access, unmatched titles, letter jumps and missing critics scores |

The inventory pass checked Boolean aggregates and cross joins across the store. These
journeys cover the first group of user workflows. Collections, queues, downloads,
imports and remaining dynamic queries need their own parity coverage. Published-image
acceptance with verified TLS is tracked separately.

No schema or API changes. The checked queries use existing error logging.

## Validation

The disposable PostgreSQL 19 race gate passed all 27 shared scenarios plus the existing
server, startup and recovery checks. The initial pass found six tests with SQL syntax
failures. A recheck reached a SQLite-only query-plan assertion, which is now scoped to
SQLite. The default Go suite, vet and embedded build passed. Web assets were unchanged.
No production data or provider services were used.

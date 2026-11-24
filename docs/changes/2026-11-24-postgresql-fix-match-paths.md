# Fix series Fix Match preview on PostgreSQL

Opening the Fix Match dialog for a series asks Arc for an example episode path.
PostgreSQL rejected the lookup with SQLSTATE 42P10 because the query ordered
distinct results by a missing-file expression outside the select list.

Group by path and missing status before ordering. Arc still prefers present
files, sorts paths within each group, and returns one result per file even if
it contains several episodes. The regression runs on SQLite and in the
PostgreSQL release gate. No migration, API contract or logging change is needed.

The new test reproduced 42P10 on disposable PostgreSQL 19 and passed with the
race detector after the fix. The full SQLite Go suite, vet, web production
build and TypeScript checks, embedded build, and one pinned-Chromium Fix Match
journey passed. No owner data or deployment was used.

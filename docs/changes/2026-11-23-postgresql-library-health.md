# Fix Library Health on PostgreSQL

PostgreSQL rejected the duplicate-file query when Library Health loaded because
its sort column was absent from the grouping. Group by the item's sort title
as well as its ID and file part. The query still sorts duplicate copies by title
and ignores missing files.

No configuration, migration or logging changes. The new regression checks
ordering and missing-copy exclusion on SQLite and in the PostgreSQL release
gate. It reproduced SQLSTATE 42803 before the fix on disposable PostgreSQL 19.

The PostgreSQL 19 race gate, default Go suite, vet, web typecheck, Vite
production build and embedded server build passed. Browser and owner-server
checks were outside this validation.

# PostgreSQL background-job and watch counts

Count matching rows in background-job summaries and library watch history using SQL
supported by both SQLite and PostgreSQL. PostgreSQL rejected the previous sums of
Boolean comparisons. This could leave Activity or Scheduled Tasks with zero job counts
even as scanning continued, and break recommendations based on library watch history.

The PostgreSQL runtime check asserts pending, done and failed counts, an empty job kind,
and a completed watch in library history. No database migration, API change or logging
change is needed.

PostgreSQL 19 race-enabled verification passed with disposable stores. The Go suite,
vet and embedded build also passed. This is a server query change, so no browser checks
were run.

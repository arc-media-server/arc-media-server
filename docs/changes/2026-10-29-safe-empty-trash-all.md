# Safe manual cleanup across libraries

Scheduled Empty Trash keeps its opt-in automatic toggle and now offers Run now for all libraries.
The explicit confirmation explains loss of removed titles' history/edits and that media files
are never deleted. The client checks libraries sequentially and reports each result, total titles
removed, skipped libraries and failures. Retry targets only incomplete results. Automatic mode
and its scheduling window are unchanged; manual Run now works while that toggle is off.

The existing per-library endpoint accepts safe=1 for batch use. It retains the scanner's complete
scan proof, unchanged/readable roots, known-present-file checks, returning-file reconciliation,
exclusive library lease and mandatory safety backup. Busy or unsafe libraries return an explicit
skip; the batch never falls back to weaker individual confirmation. Entirely empty roots still
need individual confirmation through Manage libraries. Backups are per library. Leaving the
settings component stops further requests after the current library; results are page-local,
not a durable server batch. A server restart requires another complete scan before safe cleanup.

Validation: full API package passed; focused API authorization/unsafe-refusal and real-backup
checks passed after final handler changes; scanner trash tests passed under race; API/scanner
vet, production translation/app TypeScript, e2e TypeScript, embedded build and the all-library
browser journey passed. Browser coverage exercises confirmation, skipped results, retry scope
and leaving automatic cleanup off; scanner/API tests exercise actual deletion safety. All data
was temporary. No owner cleanup or production acceptance is claimed.

Logging: the scanner retains its successful cleanup summary; removed the duplicate API success
line. Backup failure logs once at the API boundary; expected cancellation returns a conflict
without an error log. Skip outcomes are ordinary results, not warning floods.

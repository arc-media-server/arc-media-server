# #350: reject invalid paging and validate complete playlist edits

The real route regressions reproduced invalid/oversized paging silently returning 200/default
results in both API namespaces. Reuse pageBounds: limits 1–500 (default 100), offsets 0–10000000,
and 400 for invalid/out-of-range values. Valid success fields and defaults remain unchanged.
Store defaults for internal callers are unchanged.

Playlist PATCH previously renamed before checking smart rules or whether the list was smart,
and accepted names beyond the creation route's 100-character limit. Validate every supplied
field before writing; count Unicode characters after trimming. Apply accepted title/rules in
one SQL update so a subsequent database failure cannot leave a partial edit. Existing single-
field store helpers remain supported. The empty/no-op patch retains its successful response.

Red regressions reproduce 400/409 responses that still changed names, and 101-character names
being accepted. Green checks also accept 100 Unicode characters with new valid rules and inject
a database-trigger failure to prove both fields remain intact. Only synthetic profiles/libraries
and temporary stores; no owner data. Client integration notes describe validation bounds.
No new logs: ordinary validation stays a client error, and write failures use the existing fail
boundary once. No per-item work, secret/body/URL logging or new background jobs.

Required checks: apidoc, Go vet and full Go tests passed. API race passed. The combined race
run hit the existing store concurrency test's 20-second context deadline under shared CPU
load; retain that failure. An isolated rerun of the entire store race package then passed
(251.889s). No production contention or sustained-load acceptance is claimed from that rerun.

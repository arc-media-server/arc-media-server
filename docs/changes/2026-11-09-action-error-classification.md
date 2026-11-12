# #350: classify action failures at the responsible boundary

Eight real-handler regressions reproduced database/filesystem failures being returned as 400
or generic provider 502, often with SQL/path text and no diagnostic. Fix Match logged the same
failure twice. Use typed deliberate validation messages and the existing internal failure
boundary for unexpected errors; cancelled requests stay quiet. Apply this to library validation,
playlist and collection additions, backup restore/delete and scheduled Run now.

Provider HTTP failures have a separate metadata RequestError marker, so failed upstream fetches
retain 502 while matching/database failures return 500. Preserve not-found and cancellation
identity. Reuse #356's categorical transport redaction and #357's identical request failure-log
marking foundation. Canonical HTTP status text and omission of TheTVDB login-message echoes
prevent untrusted error text exposing credentials; both echoed before the regression fix.
Search errors log item/provider context rather than raw search input. Existing legacy trace
cleanup remains #356's responsibility.

Backup name/content/version validation stays 400, a missing selected backup stays 404, and
storage or safety-backup failures become generic 500. Tag source-not-found separately so a
missing safety-backup destination is not mislabeled as an absent selected source. If the DB
was restored but runtime reload fails, retain the clear Restart Arc message, keep internal
error detail in one diagnostic, and do not tell the user the restore never happened.

Preservation checks cover ordinary validation, missing/corrupt backups, provider 401 as safe
502, no duplicate diagnostic, cancellation/not-found identity and restore reload failure.
Only synthetic profiles, temporary stores/backups and in-memory/local TLS provider fixtures.
The TLS fixture trusts its test certificate and dials only loopback; no remote provider keys
or owner data. Preservation checks cover the original not-found/cancellation identities and
invalid filter syntax without banning syntactically valid future codec identifiers.


Logging is once at the failure boundary with request ID and safe operation/provider context;
ordinary validation/cancellation is not a server error. No new successful poll/per-item logs,
background work, request-body/SQL-argument logging or raw URLs. Retain #365's catalog leases,
#375's atomic library edit and #384's atomic playlist edit when resolving overlapping files.

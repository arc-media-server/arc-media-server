# #350: do not repeat unchanged failed inspections on every scan

The process-count regression reproduced the same failed ffprobe invocation on both scans of an
unchanged file. Keep a recent-failure hint for 15 minutes after a stored failure, so normal
scans still check file changes and mark files present without spawning the failed inspection
again. Saved ProbeError details remain available in Library Health. Deferred files do not claim
new successful/probed/reused details or show an active inspection that did not happen.

Hints require matching file id/path, size, precise modification time, mode, binary path and a
still-stored error. Changed fingerprints, a different ffprobe path, expiry and process restart
allow retry. Explicit Analyze bypasses the hints; successful analysis clears them. Do not cache
request cancellation or an unstable source/binary during the attempt. No permanent negative
result, schema migration, timer or new user setting.

Memory holds at most 4096 hints. Keep existing unexpired entries at capacity; overflow failures
use normal inspection behavior. Expired hints are swept at most once per minute when capacity
is needed. This is a bounded in-process backoff, not persistent scheduling or an exact-time
retry guarantee. In-place tool repairs can be retried immediately with Analyze or after the
short backoff; an ordinary later scan retries expired failures.

Tests count actual failed helper subprocesses across scans, expiry, timestamp changes,
changed tool path and explicit Analyze/repair. Source-byte guards stay intact; only fixture
timestamps and separate control scripts change. Additional portable tests cover saved-error
state, concurrent bounded memory, expiry reclamation and cancellation. The subprocess fixture
is POSIX-only; no owner files, production keys or physical Windows deployment were used.

No per-file logs or info flood: one debug-only scan summary counts deferred failures. Existing
scan outcomes and stored inspection diagnostics remain. No raw URLs, credentials, request
bodies or SQL arguments are added to logs; normal successful scans keep their existing path.

Validation: process-count regression failed before the fix; corrected focused tests, API-doc generation, Go vet, the full Go suite and serial scanner/store race checks passed (scanner 188.568s; store 232.846s). An intermediate timestamp-change fixture incorrectly rewrote protected fixture bytes and was corrected to change only timestamps; the failed evidence remains retained. No physical Windows or production-storage validation is claimed.

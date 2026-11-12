# #350: retry transient webhook failures

Confirmed the loop's err==nil admission stopped after the first HTTP/transport failure. Retry up
to three total attempts with the existing five/ten-second backoff for transport, 5xx and 429
failures. Do not retry ordinary 4xx or invalid requests. A recovered delivery clears the prior
error; each request uses a fresh payload reader. Backoff obeys context cancellation.

Regression cases failed first, then passed for recovery, exhaustion, rate limiting, fail-fast
400 and transport errors, with the backoff schedule checked without wall-time sleeps. The real
HTTP delivery test remains. Cancelled delivery produces no failure warning. Last-delivery status
still exposes success/failure; terminal failures have one warning with attempt count and elapsed
time, no per-attempt logs. Transport/invalid-request errors and HTTP status text are categorical
so credentials in URLs or remote reason phrases cannot enter diagnostics. Privacy/cancellation
boundaries are tested with fake tokens only. No payload bodies, raw URLs or SQL arguments logged.

No API/schema/UI changes. Existing 15-second HTTP attempt timeout and caller deadline still
bound delivery; this is not a persistent delivery queue or an exactly-once guarantee. When merged
with the separate secret-error PR, preserve both safe error text and retry-loop behavior.

# #350: provider-key checks do not hold the settings reader lock

Reproduced through a temporary real Arc process and gated local TMDB fixture: a key-validation
request blocked settings reads and an unrelated save; the old request then returned success
instead of noticing the competing settings intent. No owner system or credentials used.

Pass the expected settings snapshot internally from setup/PATCH to the apply callback. Validate
changed provider credentials outside the reader lock with one shared 30-second context budget.
Before validation and again before the serialized save/apply, compare current settings with that
snapshot. A concurrent successful save/restore makes stale work return 409; reload/retry instead
of overwriting it. The snapshot is internal Go state, no new client revision field or config knob.

Real-process regression failed first, then passed: settings/library reads and unrelated save work
while the provider is held, stale validation returns 409, and the newer server name remains. The
test uses isolated stores, a local fixture and prepared tools, shuts down its child, and checks
subprocess race diagnostics. No performance or physical deployment acceptance inferred. Local
binary/listener checks and database/runtime commit remain serialized; this specifically removes
slow online provider validation from the shared reader lock.

No new successful-request or credential logs. Concurrent change is ordinary validation, not a
server error. Existing provider/save boundaries remain; no full settings snapshots, keys/tokens,
bodies, raw URLs or SQL arguments are logged. Regenerate additive OpenAPI 409 responses with
`go run ./tools/apidoc`; public request/response fields stay intact.

# Logging policy, two-hour detail window and bounded stall diagnostics

Owner requested taking over Claude's proposed logging rule and extending the detail window beyond
30 minutes. Default detailed logging is now two hours; API docs, UI text and fake-time expiry/
renewal tests agree. The existing server-supplied expiry display and bounded retention remain.

Added shared guidance and a PR checklist covering lifecycle/outcome logs, actionable failures,
expected cancellation, debug-only detail, expensive argument guards, privacy and bounded volume.
This sets the review policy; it does not claim all existing feature logging gaps are filled.

Selected library and watch-state requests now expose stages and a once-per-request warning if
still running at ten seconds, limited to five such warnings per minute and 64 tracked requests.
Database pool statistics are read without SQL, so an unavailable writer does not block collection.
The admin ZIP adds bounded active-request and on-demand grouped goroutine call-stack snapshots.
No raw URLs, bodies, SQL arguments or stack argument values are added. See `docs/LOGGING.md` for
exact scope, retention/privacy boundaries and remaining limitations.

This is diagnostic instrumentation, not a fix or confirmed explanation for the owner's library
hang. It changes neither SQL locking nor request timeouts, scan admission or media files.

Validation on the prepared cloud tools (Go 1.27.0, Node 24.19.0, pinned Chromium):

- `go vet ./...` and ordinary `go test ./... -timeout 5m`: passed.
- New stall/fast-path/bounds/privacy tests and the existing slow-request test passed under
  `-race`; `internal/logbuf` race tests passed, including two-hour expiry/renewal.
- The broader `-race ./internal/api ./internal/logbuf` attempt did not complete: after roughly
  150 seconds its API process was stopped with SIGQUIT to inspect the wait. The stack was in
  the existing TestHomeHeroPreferenceIsPersistedAndProfileScoped test's SQLite fixture migration,
  not the new request tracking path. Cause not established; no full API race pass is claimed.
- UI production build, app/e2e TypeScript and 20/20 translation checks passed. Existing Vite
  configuration/dynamic-import warnings remain.
- Pinned Chromium: 3/3 selected journeys passed without retries (diagnostics including two-hour
  status/expiry/download/restart/admin boundaries; library scan controls; rating/watchlist/refresh
  recovery). Full browser/container suites and production reproduction were not run.
- Embedded-server build and `git diff --check`: passed.
- Local empty-handler microbenchmark, three 200ms samples: tracked request 1.29–1.41 microseconds,
  712 B/8 allocations; untracked control through the same middleware 0.59–0.62 microseconds,
  48 B/2 allocations. This isolates tracking cost; it does not measure loaded-server throughput,
  filesystem logging, real media I/O or on-demand runtime stack capture.

No tools/dependencies were installed, no production data was accessed and no persistent server
was started. The PR improves diagnosis; owner-server hang reproduction remains outstanding.

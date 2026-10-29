# Make manual maintenance requests and results visible

Optimize database and cache upkeep now expose an accepted manual request, queue/wait reasons,
running state and retained result in Settings. Requests outside the automatic maintenance window
are still visible. Repeated Run requests coalesce, and Run is disabled while its request is pending.

Foreground preemption retains a manual request so it can retry when playback/scanning permits.
Explicit Stop clears that request and records Stopped without a server error; automatic weekly
upkeep is deferred for a week from Stop, with Run now available to retry earlier. No SQL locking,
compaction eligibility or automatic enablement policy was changed. Last-success time survives
failures and pauses; results remain in the existing settings record, with no schema migration.

Logging adds one request/start/outcome per maintenance run (coalesced requests do not add repeated
records). Failures are logged at the manager boundary, replacing duplicate callback warnings.
Expected cancellation is an informational pause/stop. There are no per-file/query/progress logs.

Validation: background/store regression tests, focused upkeep tests including manual queue,
preemption and explicit Stop, UI build/20 translation checks, app/e2e TypeScript, and two pinned
browser journeys passed (maintenance states and diagnostics/admin boundaries). Background race checks, focused vet, embedded build and diff checks also passed. Isolated stores/generated fixtures only; no owner database,
real GPU or container/production validation. Full browser suite was not repeated for this change.

# Immutable scan completion outcomes

CQR07 unifies discovery/manual and scoped-series finalization. It records finished/error/stopped state under the scanner lock and copies the completed run before queued discovery can replace live status. OnFinish now receives that immutable snapshot; webhook selection and scan timestamps use it rather than rereading the next run. Queued watcher requests, catalog reservations, cancellation and worker joining remain. Empty trash retains its separate completion policy and emits no scan notification.

A queued-discovery regression failed before the fix with the next running job's ID, start time and empty error. It now checks that the completed scoped run is retained without assuming callback arrival order. Existing changed-folder-at-completion, Stop, scoped refresh, trash and failed-scan logging checks remain.

Go vet, full Go tests with serial packages, focused scanner/API races, and the final snapshot race regression pass. The callback signature was updated in dependent API fixtures; an initial full check caught an omitted test assignment. An initial log rename was rejected by the established failure-log test; the scan failed event is retained with added static operation context. API response contracts and media files remain unchanged.

Failures use the existing bounded outcome log with the completed job's phase/counts; cancellations are expected. No info-level file/progress loop, raw URL, key, token or SQL-argument logging added. Temporary fixtures only; no production scheduling/deployment acceptance.

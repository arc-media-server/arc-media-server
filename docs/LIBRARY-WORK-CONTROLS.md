# Explicit library repairs and safe Stop (#183/#184)

Settings → Manage libraries keeps efficient Scan/Scan all for discovery. Each
library's actions menu separately offers **Refresh all metadata** and **Reanalyze
files**. The confirmation explains scope/cost and points to Activity. Admins only.

Refresh all visits every known movie/series regardless of metadata age or normal
retry backoff. It reuses existing movie/show refresh logic, library language/provider
rules and locked-field/identity preservation. It does not discover files. Home-video
libraries have no online-refresh action. Existing item/show/selected-item refresh
and due-only scheduled Run now retain their separate meanings.

Reanalysis visits known present file records without a folder walk. It rereads
basic technical details and retries the normal bounded HDR/audio format inspection,
even if a prior optional attempt was unavailable. Files whose actual bytes changed
or became unavailable are reported as failures, retaining saved details; use Scan
to discover changed source revisions. A successful save verifies the stored revision
again and preserves file IDs, item links, presence/last-seen values and existing
probed-at timestamps. It replaces extracted streams/chapters, not edited markers,
metadata or watch history. Existing files are not made to look newly added to
background processing. Optional format evidence can still be unavailable after
another attempt; completion is not proof of physical HDR/Atmos output.

Both library-wide repairs use ID cursor pages, not a first-page limit or an offset
that changes when metadata changes. They reserve the same per-library lease as
discovery and scheduled metadata work, returning Conflict instead of overlapping
or silently replacing another run. Manual repairs are independent of automatic-task
enable switches/window settings. They report current titles/files, partial completion
and failures in Activity; final failures also appear beside the library. The normal
discovery/automatic refresh policies are not broadened by these explicit actions.

**Stop** is available beside running library work and on cancellable Activity tasks:
scans, scanner-owned metadata refresh, library repair and existing bulk refresh.
It cancels that run's context, HTTP/probe subprocess work and scanner-owned queued
follow-ups/retry timers. Stopping stays distinct from Stopped until workers drain.
Completed changes are kept; a partial discovery never marks unvisited files missing.
Reanalysis never performs missing-file reconciliation. Stop is not a permanent
change to automatic scanning: a new explicit request or future trigger may run.
Other task kinds keep their existing controls; backups are not given an unsafe Stop.

Library Stop includes the current job ID; Activity Stop targets its task ID. IDs are
seeded independently per server lifetime and remain exactly representable in the
browser. A delayed/stale request cannot cancel a later run. Repeated requests for
the same stopped run are harmless (Activity retains a bounded recent history).
Unknown/finished/stale targets return a conflict rather than stopping current work.

There is no new persistent manual-job queue. Restart retains completed database work,
not an assertion that an interrupted manual repair resumed or finished. Request it
again if needed. IDs from before restart cannot target new jobs. Existing bulk/item
refresh admission outside library-wide jobs is not redesigned by this change.

Admin API additions (see generated API.md):

- `POST /api/libraries/{id}/work` with `operation: refresh|analyze`.
- `POST /api/libraries/{id}/stop` with that run's `job_id`.
- `POST /api/activity/{id}/stop` for a cancellable Activity task.
- `GET /api/maintenance/window` for actual server time/zone/offset and open/next window.

The clock uses the scheduler's actual server-local hour policy. It handles overnight,
all-day and DST-skipped/repeated hours; it does not promise tasks run immediately
when the window opens. The UI uses server-derived display values, polls while open
and refetches after window edits, with local Retry on failure. No cron/timezone editor.

Plex references: [library actions](https://support.plex.tv/articles/200392106-library-actions/),
[scan versus refresh](https://support.plex.tv/articles/200289306-scanning-vs-refreshing-a-library/),
[Analyze](https://support.plex.tv/articles/200289336-analyze-media/) and
[scheduled tasks](https://support.plex.tv/articles/201553286-scheduled-tasks/).
Exact Plex cancellation internals are not inferred. Actual Arc results and boundaries
are recorded in READINESS.md, #183/#184/#196 and the cloud evidence folder.

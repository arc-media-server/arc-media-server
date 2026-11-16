# Library analysis and scheduled work

Owner-approved plan, 23 October 2026. Baseline 7f98b21. Implementation and verification
tracked in [GitHub #164](https://github.com/arc-media-server/arc-media-server/issues/164); source data and owner deployment are outside this cloud task.

## Behaviour

1. Discovery remains independent: automatic changes, partial scans, startup and manual scans.
   Offer Off, 15/30 minutes, 1/2/6/12 hours and daily. Preserve saved hourly settings and Off.
   Basic new/changed-file probing remains automatic and essential to playback.
2. Existing-file format upgrades become their own resumable background worker. Drain pending
   files during maintenance, independently of discovery settings. The 32-row database page
   is not a run limit. One job at a time; offline files must not block other files.
3. Preview generation and local intro/credits detection share Off / During maintenance /
   When added and during maintenance. Preserve existing thumbnail per-library opt-in and
   intro/credits library scopes. Online/chapter marker lookup remains separate and lightweight.
   Existing-file work waits for maintenance; newly probed files qualify for the added policy.
4. Keep one server-local daily maintenance window, including overnight/all-day windows.
   Provide switches for existing-media analysis, known-title metadata refresh and automatic
   backups, plus Run now for each enabled task. Manual/safety backups remain available.
   Settings › Maintenance & diagnostics › Scheduled tasks lists each task once (#202): its
   choice or switch, live status and Run now share a row. Playback › Intro & credit detection
   keeps only marker sources (chapters/IntroDB, TheIntroDB key) and links to the task's timing.
5. A shared admission controller serializes optional heavy analysis/preview/detection work.
   Playback and scanning take priority. Cancel optional subprocess work when its window ends,
   settings disable it, playback/scanning begins or Arc shuts down. Resume unfinished work
   later; finished revision-specific jobs persist across restarts. Failed jobs use bounded
   retry delay and explicit Run now can retry them. Keep source files read-only.
6. Persist job state/revision and expose waiting, running, paused, completed/failed status through
   Scheduled tasks and Activity. Run now bypasses the clock window for the current backlog,
   respects disabled tasks and foreground priority, and coalesces repeated clicks.

## Data and compatibility

- Migrate existing hour intervals to a canonical minute setting; retain legacy hours API keys.
- Read legacy preview-night and local-detection booleans when new task modes are absent. New
  canonical modes win when both are present. Explicit false/Off must survive saves/restores.
- Persist an analysis-policy epoch so old-library work stays scheduled while new work can run
  on addition, across restarts. Revision keys prevent stale completion after a file changes.
- Restore reloads modes/window immediately. In-progress work is checked against live policy.
- Existing completed thumbnails/markers remain reusable; no forced regeneration of the library.
- No cron editor, concurrency knobs or media deletion. User-requested optimized conversions
  and essential playback work remain independent.

## Implementation order

Settings/schema and durable job records; shared admission/status/Run now; independent format
worker; preview/detection integration; API and grouped Settings/Activity; isolated regressions
and full cloud verifier; shared handoff and verified direct publication to main.

## Acceptance

Controlled-clock checks cover normal/overnight windows, added versus existing work, Off,
foreground pause/cancellation, restart/revision persistence, failed retry, Run now and setting
changes. A backlog larger than 32 must drain without another scan. Verify strict API validation,
non-admin denial, legacy migration, restore, 15/30-minute scheduling and no duplicate timers.
Browser checks cover scheduling choices, task status/actions, save errors, persistence and
narrow layouts. Run production/app/e2e TypeScript, Go vet/tests/race, embedded build and all
browser journeys using temporary stores, generated media and local providers only.
Record actual failures and final results. Physical NAS scheduling, GPU/device output, Unraid
and sustained-use acceptance remain separate. No persistent Arc server or owner data required.

References: Plex Library settings and Scheduled Tasks documentation describe the comparison
model; Arc defaults and precise cancellation/queue behaviour are defined above.
https://support.plex.tv/articles/200289526-library/
https://support.plex.tv/articles/201553286-scheduled-tasks/

## Implemented contract

Rescan separation follow-up (#169): age-only metadata refresh and old unmatched retries are
maintenance work. Movie maintenance drains cursor pages rather than a fixed per-scan allowance;
failed candidates are attempted once per pass. Existing unknown episode metadata retries in
maintenance after an hour; genuinely new unknown files can request fresh data during discovery.
Explicit language changes mark known unlocked titles pending so their requested refresh remains
independent of scheduled maintenance. Details: [INCREMENTAL-RESCAN-PLAN.md](INCREMENTAL-RESCAN-PLAN.md).

Migration 0048 persists revision-specific jobs and a policy epoch. The epoch separates old
library files from newly probed files for the added policy; restart does not reclassify old
files as new. Revisions include file identity/path/size/time; local detection also includes
an explicit marker-reset generation, so negative results are remembered until files change
or the owner requests a reset. Deleted media records cascade their job records. Existing
preview artifacts remain reusable; a missing artifact reopens its completed preview job.

The analysis worker drains cursor pages with a single lease at a time. Optional stream/frame
format probing keeps its existing four-second budget. Waiting worker types receive fair turns;
a completed/no-longer-needed pass releases its admission request. Paused subprocess leases are
cancelled within a 250 ms policy check, and wake on schedule/foreground transitions. Essential
new-file probing and user-requested playback/conversion stay independent.

Run now is a coalesced, in-memory override for the backlog present when requested, not a saved
change to the schedule. Completion clears it. Restart/restore returns unfinished work to the
normal configured schedule; completed revision records remain in the database. Failed jobs
wait an hour before another normal attempt; Run now can retry failures immediately. An
optional source-format sample saved as unavailable remains a completed bounded attempt,
not a recurring full-file probe.

Metadata refresh pauses with foreground work or a closing/disabled maintenance window, then
continues by checking title freshness. Backups finish an admitted database copy safely;
automatic admission respects the switch/window/foreground policy, while manual and safety
copies remain available. Online/chapter marker lookup keeps its separate lightweight cadence;
Run now for local detection does not trigger external metadata lookups.

Task counters describe recorded queue jobs, not a count of every undiscovered media item.
Discovery Off never enables a hidden discovery walk. Saving unrelated settings does not run
backups or force known-title refresh. State/status is admin-only. Local/runtime scheduling
acceptance is separate from NAS throughput, real-device, Unraid and sustained-use evidence.

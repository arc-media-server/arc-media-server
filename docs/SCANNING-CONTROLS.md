# Global scanning controls

Status: implemented locally, 22 October 2026, following the owner's approval to implement.
Global/server-wide controls replace the earlier per-library proposal. The following contract
records the chosen defaults, behaviour and verification requirements. See READINESS.md for evidence.
Tracked in [GitHub #147](https://github.com/JonzaUK/arc-media-server/issues/147).

## Placement and everyday flow

The **Library scanning** section lives under admin Settings > Libraries & metadata
(`?section=library`). Scheduled tasks live under Maintenance & recovery.
Its introduction reads: “Keep all your libraries up to date.” Settings apply to movies, TV and
other-video libraries. Do not add per-library overrides in this first version.

Use the existing settings layout, immediate save behaviour, inline errors and narrow-screen
layout. Changes take effect without restarting Arc. Ordinary profiles cannot change these
settings, including through the API.

| Setting | Recommended default | Help text and behaviour |
| --- | --- | --- |
| Scan automatically when files change | On | “Find new or changed media when your folders report a change.” Independent of scheduled scans. |
| Full library scan | Every hour | “Check all library folders for changes, including changes your drives may not report.” Choices: Off, every 15/30 minutes, every hour, every 2/6/12 hours, daily. |

Under a collapsed **Advanced** disclosure:

| Setting | Recommended default | Help text and behaviour |
| --- | --- | --- |
| Scan only changed folders | On | “For detected changes, check the affected folders instead of the whole library.” Disabled when automatic change scanning is off; retain its saved value. |
| Scan when Arc starts | On | “Check all libraries for changes when the server starts.” Independent of the full-scan interval. |

Retain each library's existing **Scan now** action regardless of these settings. Adding a new
library still starts its first scan; changing its folders still checks the newly configured
content. These are explicit user actions, not automatic scheduled scans.

## What Off means

Turning off automatic change scanning stops watcher-triggered scans and discards pending
watcher-only changes; it does not cancel an already running scan. Scheduled scans remain active
if a full-scan interval is selected. Turning automatic scanning back on requests one catch-up
scan per library, rather than replaying a backlog of old events.

Selecting Off for Full library scan stops recurring whole-library discovery, including the
current unconditional nightly discovery walk. It does not turn off change detection, startup
scans, explicit user requests, or bounded retries finishing work from an already requested scan. An explicit backup restore also reconciles restored libraries.
Show concise contextual text identifying any automatic discovery still enabled.

With all three discovery triggers off, say: “Libraries update when you scan them or change
their folders.” Do not quietly re-enable a timer or perform a nightly discovery scan.

Backups, scheduled intro/credits detection, configured thumbnail work and refreshes of known
titles remain governed by their existing maintenance settings. Refreshing known titles must
not require a full media-folder discovery walk. Explicit Refresh metadata remains separate
from Scan now; a scan does not force every provider response to be downloaded again.

## Network shares and unavailable folders

Show a short note beside the schedule: “Some network drives do not report changes. Keep a
scheduled scan enabled to catch them.” Do not claim to detect reliable notifications from
the filesystem type alone.

If watching actually fails or loses events, show the affected library in Library Health with
the reason and whether a scheduled fallback is enabled. When scheduling is Off, explain that
Scan now or enabling a schedule is needed; do not promise an automatic fallback that is off.
Watcher-error reconciliation counts as change-triggered work and must respect its toggle.

Retain existing unavailable-root and incomplete-walk safeguards. An offline share must not
erase titles, edits or watch history. Reattach watchers to returning roots using lightweight
availability checks independent of the full-scan timer; these checks must not become hidden
recursive discovery scans. Existing Scan now remains available while a drive is offline.

## Scheduling and work already in progress

- Preserve the independent 10-second settling period; it is an internal default, not a knob.
- Partial scans apply only to detected changes. Scheduled, startup and manual scans reconcile
  whole libraries. Root-level changes and event overflow may require a full reconciliation.
- Use intervals from server start, resetting the timer when its saved interval changes.
  Daily means every 24 hours, not a selected clock time; label the behaviour accordingly.
- Saving settings must not start a scan merely because the interval changed. Off removes the
  pending periodic timer. Do not replay missed timer ticks after a slow scan or downtime.
- Coalesce requests per library: no simultaneous scans of the same library or accumulating
  repeated full scans. A full request includes pending partial scopes; later genuine changes
  are retained for a follow-up pass. Preserve cancellation and provider retry backoff.
- A settings change leaves an in-progress scan running. New automatic requests use the latest
  settings. Turning settings off is not a Cancel scan action.
- Restore must reload this configuration immediately, as other runtime settings do. Switching
  intervals, restoring, or changing unrelated settings must not duplicate timers/watchers.

## Implementation boundaries

Persist global values through internal/settings and the existing admin settings API; accept
only the supported intervals and actual booleans. The canonical scan interval is stored/API-exposed
in minutes (`scan_interval_minutes`); legacy hour requests remain supported, including fractional
hours for the 15/30-minute choices. Missing values load the recommended defaults
for existing installations: automatic On, partial On, hourly, startup On. Keep explicit false
distinct from an absent value. No configuration file or per-library schema is needed.

Update internal/watcher to accept live configuration, separating watch registration, change
dispatch, root reattachment and periodic reconciliation. Wire settings changes and restore in
cmd/ams/main.go. Refactor nightly known-title refresh away from its current full-scan request
before claiming schedule Off is honoured. Update the Scheduled tasks help text, which currently
promises that every library is checked nightly.

Continue using Activity for actual running scans and Library Health for actionable failures.
No additional dashboard, worker/cache controls, cron editor or custom scheduling UI is needed.

## Verification checklist

- Isolated settings/API tests: missing defaults, explicit false, invalid intervals/types,
  persistence, partial patches and non-admin rejection.
- Controlled-clock watcher tests: independent trigger combinations, partial/full routing,
  runtime timer replacement, no duplicate/catch-up tick backlog, pending events on disable,
  re-enable reconciliation and watcher failure with scheduling on/off.
- Integration regressions: startup off, first library addition still scans, manual scanning
  still works, nighttime maintenance does not discover files with all triggers off, known-title
  refresh/backups remain enabled, restore reloads settings, offline/returning roots preserve
  identity and watch state. Use temporary data and local provider fixtures only.
- Browser journey: admin settings save and survive reload, dependent Advanced control states,
  schedule Off explanatory text, error recovery, ordinary-profile access and narrow layout.
- Run the required Go, UI and browser checks. Record real network-share behaviour separately;
  simulated watcher failures do not establish Unraid or sustained-use acceptance.

## Plex reference and deliberate differences

Plex places automatic/periodic scanning and the advanced partial-scan switch in global
[Server > Library settings](https://support.plex.tv/articles/200289526-library/). It documents
network notification limits and intervals from server start. Arc follows that placement and
separation, with one schedule dropdown instead of an enable switch plus interval dropdown.

Arc now supports the same 15/30-minute and 1/2/6/12/24-hour interval choices (#164). Hourly
retains Arc's missed-event fallback and is not asserted to be Plex's default. The Advanced
startup switch makes Arc's existing behaviour explicit. See BACKGROUND-WORK-PLAN.md for
independent media-analysis and optional-processing schedules.

Plex separately exposes an optional maintenance-time discovery scan in
[Scheduled Tasks](https://support.plex.tv/articles/201553286-scheduled-tasks/). Arc's proposal
uses one recurring discovery schedule, avoiding a second hidden full scan after choosing Off.
These simplifications were recorded on decision issue #80 before implementation.

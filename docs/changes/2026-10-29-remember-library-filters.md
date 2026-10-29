# Remember library filters with sort and direction

Owner requested keeping library browsing filters after leaving the screen. Library filter changes,
chip removal, Clear filters and sort/direction changes now save the durable controls together in
browser storage, scoped to the active profile and library. Existing sort-only preferences continue
to load. Returning through the sidebar restores remembered controls; Back/Forward and reload retain
explicit URL state. The existing active filter count, chips and Clear filters remain visible.

An explicit library query (for example a genre/person/Home link) wins without merging remembered
filters. Merely following that link does not overwrite the remembered view; editing its controls
makes the current filters/sort the remembered view. Clear filters immediately updates storage while
retaining sort/direction. In-page URL replacements, including consumed letter jumps, do not restore
old filters. Library tabs, letter jumps, paging, bulk selection, dialogs and scroll position are not
saved as durable preferences; their existing behaviour remains.

The Profile menu clears the current library query when deliberately switching profile or signing
out, so the next profile restores its own state. Initial sign-in deep links are untouched. Storage
reads are allowlisted and basic values validated; missing/invalid/unavailable browser storage falls
back to working URL controls. No server preferences, database migration, external calls or cross-PC
synchronisation are added.

Logging review: this is browser-local preference state and creates no background job or new server
failure boundary. No routine logs were added: logging filter values on every navigation would add
noise and could disclose browsing choices. Existing request/error feedback remains, and focused
browser checks cover storage failure and profile/navigation boundaries.

Validation (prepared Go 1.27.0, Node 24.19.0 and pinned Chromium, starting main b14bda5):

- UI production build, app TypeScript and e2e TypeScript passed; all 20 translation checks passed.
- Six focused browser journeys passed together (46.8 seconds), zero skips/retries: movie and
  series library leave/return, Back/reload/clear; library and actual UI profile-switch isolation;
  explicit query precedence and durable clearing of empty results; invalid/unavailable storage;
  and existing per-profile sort direction.
- The browser harness built a disposable embedded server against generated media and local
  provider fixtures. `git diff --check` passed. No Go source changed; full Go/race/browser/container
  suites, cross-PC synchronisation, production and physical-device checks were not run.
- Initial new-test runs used an outdated switch label, then an immediate checkbox-state check
  during controlled URL navigation, then a poster-centre click intercepted by its Play overlay.
  Corrected the tests to use the current label, click plus awaited state assertion, and visible
  title text. The final six-pass run retained all assertions; no runtime failure was hidden.
- Existing Vite configuration/chunk/dynamic-import warnings remain. Prepared npm dependencies were
  reused through a temporary symlink; no tools/dependencies were installed and no persistent
  server or owner data/settings/media were accessed.

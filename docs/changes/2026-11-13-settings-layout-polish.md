# Clearer Settings sections and quieter save feedback

Settings pages use the selected section name as their main heading. Desktop sidebar groups
have spacing and divider lines; mobile keeps its grouped section selector. Remove the repeated
save-instructions paragraph and completed automatic-save labels. Pending and inline failure/
retry feedback remain, along with explicit Save buttons for text fields.

Manage Libraries now introduces library management directly and omits the repeated scan-help
paragraph. Scan actions, running guards and error feedback remain unchanged.

No API or server behaviour changes. Existing request failure logging remains sufficient;
this presentation change adds no logging or hot-path work.

Validation: production web build (including TypeScript, 3 i18n and 3 unit test files), e2e
TypeScript, and 5 affected browser journeys passed. Browser checks cover all five Settings
headings, desktop/mobile navigation, Manage Libraries scanning, pending/error/retry feedback,
persisted settings and profile save recovery. Used pinned Chromium, generated media, local
provider fixtures and temporary stores; inspected the desktop screenshot. No production or
physical-device acceptance claimed. Existing Vite config/import warnings remain.

Dashboard now prefixes its title with the configured server name (falling back to Dashboard
while unavailable). The Activity popover places View Dashboard first, above the Activity
heading and scrollable tasks. Remove the old Server heading from dedicated Settings pages.
The follow-up production build and e2e TypeScript passed. Both affected browser journeys
passed again, checking the configured name, Dashboard link above Activity, mobile navigation
and absence of the old Server headings. No server implementation or logging changes.

Temporary folder now uses a full-width clickable path box. It shows the saved path, or a
clear default placeholder when no override is stored. Clicking opens the chooser at the
current folder; Use Default clears the override. Pending saves disable the controls and
errors retain the existing path. No change to the server's temporary-storage behaviour.

General is now a dedicated Settings destination under Server, containing Server identity
(name) and Notifications/webhooks. Notifications move out of Libraries & metadata; sign-in
and network/device controls stay under Access & devices. Desktop links and the mobile
selector use the same grouping. An existing pre-roll error-visibility test now centres its
control before triggering an error so the inline feedback has viewport room.
Final validation after the folder/General changes: production build, app/e2e TypeScript,
3 i18n and 3 unit test files passed; 6 focused browser journeys passed (49.6s), including
notification failure/empty/stale recovery, navigation, inline save recovery, folder chooser/
cancel/persistence/reset/failure handling, and server identity. Inspected the mobile folder
screenshot. The preceding browser run passed the new folder journey but exposed the existing
pre-roll test's viewport assumption; its setup was corrected before the final passing run.

Reviewed headings, controls and help across General/notifications, profiles, libraries and
editor, playback, access/devices, maintenance/diagnostics, Library Health and Backups. Clarify
provider credentials without free-access/quota promises, scheduled scans and Off states,
conversion limits and HDR support, home-network versus password-bypass rules, proxy HTTPS,
maintenance windows and automatic trash removal. Update stale device navigation and Health
playback/inspection/retention wording. Retain specific backup/restore scope and readable
controls. Names such as Scheduled library scans, Video conversion, Import from another
server and Autoplay next episode now explain the task. Translation/section keys, routes,
API fields and defaults remain unchanged; exact-text browser locators follow the labels.
Detailed review notes are in the cloud evidence folder (COPY-REVIEW.md). No logging changes.
Copy-review validation: final production build, app/e2e TypeScript, 3 i18n test files and
3 unit test files passed. Of 13 focused browser journeys, 11 passed initially; two expected
old scan-help/Saved text. Those assertions were updated to the intended UI and both passed
on recheck (36.4s). No unresolved browser failures; no Go implementation, API contract or
setting-default changes. The final build also includes the follow-up Health helper text.

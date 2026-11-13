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

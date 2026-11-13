# Remove playback estimates from Library Health

Library Health no longer shows the device-based Playback compatibility report or issues its
per-library compatibility requests. Remove the unused report component, device presets, frontend API wrapper/report type and
their translations. Library
problems and marker coverage remain; the compatibility API remains available to integrations.
No server behaviour or logging changes are needed for this UI removal.

Validation: production build, TypeScript, three i18n and three unit test files passed.
The affected desktop/phone Settings navigation browser journey passed (1 test, 29.7s)
using pinned Chromium, generated clips, local providers and temporary stores. The initial
build caught an unused translation, which was removed; the final build passed. Restricted
browser startup failed, so the successful run used the permitted local-server execution path.
No Go code changed; the browser fixture compiled the embedded server successfully.

Follow-up: removed the remaining report-only frontend helpers and device-name translations.
The full web build (including TypeScript, i18n and unit checks) passed again after cleanup.
The browser result above predates this unused-code-only follow-up; it was not repeated.
The published server route, API contract and shared playback decisions are unchanged.

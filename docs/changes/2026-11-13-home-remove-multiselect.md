# Keep bulk selection in library views

Remove Home's partial row/library-scoped multi-select controls and selection bar. Home cards
retain Play, administrator Edit and per-title menus across poster, thumbnail and backdrop
styles. Library multi-select is unchanged. Delete the unused Home selection context, state,
keyboard listeners and scope text rather than only hiding its trigger.

No server/API/default changes or additional logging are needed; per-title action failures
and edit retries retain their existing handling.

Validation: production build, app/e2e TypeScript, 3 i18n and 3 unit test files passed.
Both affected Home browser journeys passed (32.6s), covering all three card styles,
Play/Edit/menu actions, touch and ordinary profiles, retained library bulk actions and
Continue Watching removal. Used pinned Chromium with generated media, local providers
and temporary stores. No Go code changed; fixture server built successfully. Existing
Vite config/import warnings remain. No production/device acceptance claimed.

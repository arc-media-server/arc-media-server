# Home card actions

Home rows reuse the library cards' Play, Edit and More actions. Administrator selection uses the
existing SelectionBar and bulk API, scoped explicitly to loaded movie/show titles in that row
and one library. Only one row selects at a time; Escape/Done exits. Episode actions target the
episode and do not offer movie/show bulk merge. Grouped season cards retain their show identity.
Continue Watching keeps resume and Remove from Continue Watching behaviour. Touch devices keep
shared controls visible. Ordinary profiles retain playback/personal menus without Edit/Select.

Validation: production translation/app TypeScript build, e2e TypeScript, embedded server build,
all 36 UX journeys passed, including playback/queue/filter/settings regressions. The new Home
journey additionally passed after moving to its own file; it covers editing, shared More,
actual bulk watched update, loaded-row scope, touch visibility and ordinary-profile permissions.
No owner data or physical-device acceptance. No new server logs: existing bulk/playback/menu
endpoints and their failure feedback are reused; no per-card/poll logging was added.

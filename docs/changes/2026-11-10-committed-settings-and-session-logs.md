# #350: log committed settings safely and report stale-session cleanup

The real settings route regression reproduced values/URL-bearing unknown fields and unchanged fields being logged after a successful request. Remove the body-copying logging middleware. After a successful apply, compare the compiled Settings fields and log only changed known field names plus the actor's opaque profile ID. No setting values or submitted keys enter the diagnostic. Ignored, unchanged and failed saves do not claim changes. Exclude private loaded-state fields and the internal background epoch.

The settings handler retains an explicit 1 MiB decoder bound after removing the old middleware reader. This remains tighter than #396's shared authenticated ceiling. The route's original documentation comment now attaches to its actual handler; regenerated API docs gain that description without a response-schema/type change. Preserve #376's before/next optimistic apply guard when merging the overlapping handler.

The daily stale-session cleanup previously discarded its outcome. Return the actual removed-row count; record one completion with count/duration, or one database failure, per existing startup/daily pass. Expected root cancellation is quiet. The SQL is fixed and has only a time cutoff parameter; no token, device, user, SQL argument or request body is logged. This does not add jobs or change the 90-day cutoff. Broader joining of all background workers is still the separate shutdown finding.

Tests cover URL/secret-bearing known values and unknown keys, no-op/failed saves, actual session removal counts, exact cutoff preservation, empty outcomes, cancellation and a closed-database failure. All use temporary stores; no provider validation requests, production keys, owner data or persistent server. No frontend change. Full required validation results follow when checks finish.

Reflection runs only after successful saves with info logging enabled, compares compiled fields without rendering values, and adds one bounded record rather than per-item logging. Existing request errors remain at their responsible boundaries.

Validation: settings privacy/no-op regression failed before the fix, then passed. Focused settings/session outcome tests, API-doc generation, Go vet, full Go suite and serial API/store/main race checks passed (API 246.008s; store 273.102s; main has no tests). Generated docs only restore the settings-handler description; response schemas and the v1 contract remain unchanged. No web/browser changes.

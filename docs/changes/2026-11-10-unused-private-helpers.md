# #350: remove unused private scaffolding, retain active GPU detection

Re-verification corrected part of the report: Manager.detectMu, probeHardware and renderNodes are actively used by hardware detection and its regression fixtures. Keep them. The duplicate fields with those names on Session have no readers/writers and are removed. The private scanner.syncSeriesGroups wrapper also has no callers; the current scan pipeline directly invokes syncSeriesGroup, which remains intact.

The retained disabled IMDb ingestion implementation is intentional under docs/RATING-SOURCES.md's deferred owner/source decision. Do not reactivate it or remove identifiers, stored scores, compatibility APIs, caches or schema as incidental dead-code cleanup. This PR leaves that documented decision alone and records the distinction on #350.

No behavior, API shape, logs, media or dependencies change. Adding a test to assert that an unused private field/function exists would not verify user behavior; existing scan and hardware-detection tests are the meaningful controls. Required validation results follow after checks complete. No new diagnostic boundary or logging is needed for unreachable scaffolding; no secrets, URLs, bodies or SQL arguments are logged.

Validation: Go vet, full Go suite and serial scanner/transcode race checks passed (scanner 192.752s; transcode 12.258s). Existing hardware-detection fixture hooks remain active. No API or web change; no runtime failure was claimed or fabricated for removal of uncalled private declarations.

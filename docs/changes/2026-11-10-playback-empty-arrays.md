# #350: consistent empty playback arrays

Re-verification narrowed the report: ordinary playback already returns empty chapters/markers as arrays, but early playback-preview needs-analysis/no-video decisions return null lists. The real-route regressions reproduce those branches. Normalize playback list fields immediately before JSON responses, preserving the original struct types, populated entries and optional reason_codes omission. This covers normal playback, previews and early decisions without changing codecs or starting conversion work.

A profile's library_ids deliberately keeps its existing null-all versus empty-array-none behavior. Document that sentinel in CLIENT-INTEGRATION.md and protect it with a serialization regression. Do not apply playback normalization to profile permissions. The existing machine v1 snapshot still records the array shape; the guide explicitly tells SDK authors to accept the legacy null sentinel. No machine-contract type line is removed or changed.

An intermediate custom MarshalJSON made runtime tests pass but correctly caused the API generator to treat PlaybackPlan as opaque, losing schema fields. The v1 contract tests caught that. Replace it with a plain pre-serialization helper; keep the contract guard unchanged and retain the failed intermediate evidence. A first test also mistakenly required optional reason_codes when empty; corrected to allow omission while requiring arrays whenever present.

Validation: corrected before-fix regressions fail; focused real v1 preview/control/profile-semantic tests, API-doc generation, Go vet, full Go suite and complete API race checks pass (251.162s). Generated OpenAPI and the v1 contract have no diff after the ordinary-helper fix. No web change, real playback/physical-device claim or production data used.

No new logs, jobs or per-frame/per-item work. Existing request/decision diagnostics remain; this does not add secrets, URLs, bodies, SQL arguments or duplicate errors to logs.

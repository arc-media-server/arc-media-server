# API contract corrections and server identity

The API review found an omitted settings PATCH route, output encoding rules incorrectly used for
request requiredness, and obsolete per-show provider instructions. Route discovery now parses Go
registrations structurally, unwraps known decorators and fails on unknown ones. Input schemas are
separate from success outputs, with optional/default/explicit-null metadata. Settings raw-envelope
validation is followed to its actual typed request. Regression checks cover partial edits, profile
credentials, playback defaults, reset nulls, settings schema and unchanged output requiredness.

Added authenticated identity/capabilities endpoints and admin server_name patch support. Identity
is generated once by migration 0060; rename is bounded/validated and stored with existing settings.
Restore/clone identity semantics and app integration are in docs/API-FOUNDATIONS.md. No UI changes:
Claude owns the owner-approved frontend work. No new dependency or persistent server.

Logging: identity/capability reads are silent. Rename uses the existing settings outcome boundary.
There is no per-item/poll logging, raw URL/path/credential exposure or provider/media work.

Actual validation: full Go tests pass (31 packages with tests; six packages have no test files), full go vet, focused API/apidoc race checks, production UI/TypeScript build and embedded-server build pass. Generated API/OpenAPI and additive v1 checks pass. New tests cover name validation/access, persistence across rename/restart/restore and distinct-server IDs. Existing UI build emits its known dynamic-import bundling warnings. No UI behavior changed, so no browser suite was repeated. Initial generator/compile/test failures during implementation were corrected before final checks. Real deployment/device/clone acceptance remains
separate from temporary-store tests. The palette feature and previous setup evidence are preserved.

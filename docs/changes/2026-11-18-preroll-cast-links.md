# Clip-scoped playback links for native pre-roll players

Native clients can exchange a configured pre-roll URL for a link through the existing cast API.
Previously the request returned 400 "not a stream link", while direct pre-roll requests needed
Bearer authentication that native players cannot always attach. Session keys remain out of
player URLs. A pre-roll grant binds one configured path and index, exits before other stream
routes, supports Range and follows existing cast expiry/session revocation.

No API fields/routes removed or renamed. The regression failed on current main before the fix;
HTTP coverage verifies full/range delivery, invalid routes, cross-clip/item/extra/conversion
isolation, configuration reorder and revoked sessions. Final checks: go vet passed; healthy-cache go test ./... passed (37 packages, seven without tests);
all cast tests passed with -race; production web/i18n/unit/TypeScript build, API documentation
regeneration and embedded-server build passed. No web UI changes or browser journeys affected.

The first full run failed one download-revocation timing assertion and API-documentation export
checks against dangling prepared Go cache entries. The focused download check passed on changed
code and unchanged main. A separate task cache fixed export generation, and the complete suite
then passed. The broad API race run exceeded its default ten-minute package limit; only the
focused cast race suite is claimed as passing. Prepared cache files were preserved.

Client handoff: docs/PREROLL-CLIENTS.md. No owner server/media/settings/keys used. Real-device
codec, playback, proxy and deployment acceptance remain outstanding.

Decisions: extend the existing cast mechanism rather than exposing media or session credentials.
No new controls or product policy; the existing Plex-style configured all/random behaviour stays.
Logging: existing cast/request failure boundaries apply. No extra success/per-clip records and
no paths, grant tokens, session keys or raw URLs logged.

# Watch Together backend

Added private ephemeral synchronized-viewing rooms for household profiles and existing friends.
A shared timeline uses revision-checked commands and generation-checked readiness while every
participant retains normal personal playback, quality, tracks, conversion admission and history.

Expiring invite codes, own-device room recovery and full-state room SSE avoid a global room
browser or new infrastructure. Per-participant media access is rechecked; denied later titles
stay as placeholders. Buffering waits at most ten seconds before slow viewers catch up. Host
control passes to the oldest remaining member. Counts, request rates, bodies, stream slots and
lifetimes are bounded; sessions are revalidated and revoked streams close.

The existing admin Playing DTO gains only a room ID and participant count. No frontend work,
chat, reactions, server deployment or actual multi-device validation is included. Client contract,
clock/drift algorithm, converted-file position rules, UX flows, limitations and official Plex
references are in [WATCH-TOGETHER-API](../WATCH-TOGETHER-API.md). Decisions are logged on #80.

Logging: info lifecycle/joins/leaves with counts/duration/fixed reasons, debug commands and
waiting decisions. No per-heartbeat info or titles/codes/tokens/session hashes in room logs.
Validation results are recorded in the PR; fixtures never use owner media/settings or providers.

## Local verification

- Production UI build (translations, app TypeScript, Vite) and e2e TypeScript passed; no UI source
  changes. Embedded-server build and git diff --check passed.
- Final go vet ./... and go test ./... -timeout 10m passed: 33 tested packages, 6 without tests.
- Focused room/auth/API race checks passed, including concurrent commands, global detached-stream
  bounds, atomic attempts, authenticated-router SSE and revocation. API-doc/contract checks pass.
- An earlier full run and isolated recheck failed the existing TestConversionWaitsForThePlayer.
  The same failure reproduced on unchanged starting main ad2f538; the final full run passed.
  No transcode implementation/test changes or relaxed checks were made; this remains an observed
  baseline timing-sensitive test. Original failure and final success logs are retained separately.
- Browser journeys and real-device/multi-device synchronization were not run for this backend-only
  work. No actual TV playback, WAN latency, hardware acceleration or sustained-load claim.

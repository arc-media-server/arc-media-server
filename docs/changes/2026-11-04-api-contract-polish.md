# API contract and client integration polish

Completed the six API review gaps on `codex/api-contract-polish`, based on PostgreSQL #337.
Cross-origin `/api/v1` clients can resume SSE with Last-Event-ID and read X-Request-ID; cookie
and local-login cross-origin restrictions remain. API responses receive server-issued decimal
process-local correlation IDs shared with existing diagnostics. Common JSON errors preserve
error/needs/domain fields and add stable HTTP-category code/request_id; success payloads are unchanged.

Capabilities advertise admin-only database_management based on manager availability, without
probing the database. OpenAPI documents logical query types/defaults/bounds/flags, binary upload
limits, actual media types, conditional/range headers, SSE cursor/framing/typed payloads and error
metadata. Query flag wire encoding remains 1/0 where required. Unknown discovered query metadata
fails generation. No API routes were added or removed (163 paths/198 operations); all prior v1
baseline lines remain, with 11 additions documenting existing 206 responses.

CLIENT-INTEGRATION.md supplies current auth/pagination/playback/events/error guidance and executable
read-only examples. BACKEND-API-HANDOFF.md distinguishes existing APIs from independent pending PRs;
the PostgreSQL UI handoff documents capability detection. No UI/product feature work was added.

Validation: Go vet passed; full Go run had 35 passing packages, 585 test/subtest passes, zero failures,
15 PostgreSQL integration skips (no PG fixture) and six packages without tests. API/apidoc packages,
targeted race checks, final generated-contract/guide request checks, production web/i18n/app TypeScript
build and E2E TypeScript passed. Pinned Chromium: 3/3 affected journeys passed, zero retries, covering
sign-in, profile permissions and API docs. Tests used isolated stores, generated clips and local
provider fixtures. No full browser/container suite, owner server, hardware, proxy or deployment
acceptance; previous PostgreSQL migration evidence was preserved, not repeated. Existing bundler
locale import warnings remain. Detailed evidence is outside the repo at
`/workspace/arc-cloud-evidence/api-contract-polish/REPORT.md`.

Logging: add IDs to existing error/debug/slow-request records; no new per-success info, frame or
loop logs. Existing 64-request tracking cap and five stall warnings/minute remain. No secrets,
bodies, SQL arguments or new raw-URL logging. Error metadata cannot rewrite plain-text transport
errors or an already-started stream; clients must check status/content type and tolerate older
servers without the new fields. IDs are not authentication tokens or global durable identifiers.

Open as a follow-up to #337; merge predecessor and retarget before merging this PR. No PRs merged.

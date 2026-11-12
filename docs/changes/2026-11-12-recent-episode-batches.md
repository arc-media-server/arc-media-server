# Recently Added: fixed 24-hour episode batches

TV Recently Added now groups available, visible episodes by show and arrival batch before
choosing a card: one episode → episode; several episodes in one display season → season;
several seasons → series. Unknown placement in a multi-episode batch uses a series card.
New shows follow the same rules as established shows.

The newest episode anchors an inclusive 86,400-second window. Older additions start a separate
batch; members do not extend the window through a chain. Later additions can therefore coexist
with an older season/show card. Current display seasons control grouping; watches and metadata
refresh preserve arrival history. Card limits retain whole batches. Home keys include the
batch timestamp to keep repeated show/season cards distinct.

No database migration, provider lookup, scan policy or source-file change. Existing API fields,
access/availability filters, season-artwork rules, navigation and shows-only preferences remain.
Plan and client guidance: [RECENT-EPISODE-BATCHES](../plans/RECENT-EPISODE-BATCHES.md).
Arc defines 24 hours explicitly; Plex's exact internal interval was not verified.

Logging: no new records for this read-only grouping path. Existing request/store/ordering
failure boundaries remain responsible; no per-episode logs or expensive disabled-debug work.

Validation: Go vet and the full normal Go suite passed (37 tested packages). Production/app
TypeScript, E2E TypeScript, six i18n/unit test files and all 13 affected browser journeys passed
(0 failed/skipped/retries); desktop/phone captures inspected. API docs regenerated and the v1
contract snapshot retained. The broad API race package exceeded its 10-minute timeout while
an unrelated smart-collection test had been running for 1s; this check is incomplete, not passed.
Focused API batching, large-import and context-cache race regressions passed (69.104s); full
store race tests passed (401.961s). The combined broad race invocation exits 1 due to the API timeout. Tests use isolated stores, generated clips and
local provider fixtures; no owner data or physical/native acceptance. The full browser suite
was not run. Initial fixture/build/compile mistakes and the expected red baseline are preserved
in `/workspace/arc-cloud-evidence/recent-episode-batches/`, alongside final logs/captures.

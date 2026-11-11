# Client API integration index

Start with [CLIENT-INTEGRATION.md](CLIENT-INTEGRATION.md), bundled on the server at
`/docs/client-integration.html`. It covers device authentication, profile selection, browsing,
playback/progress and error handling. The generated [API reference](API.md) and
[OpenAPI contract](openapi.json) provide endpoint and model details; `/docs/` is the interactive viewer.

Use `/api/v1`, detect capabilities on connection/profile changes, and enforce the server's returned
permissions and decisions. A guide is not proof that a particular deployed server has a feature.
Keep existing shared UI controls when integrating these contracts.

## Implemented contracts

- Identity, editable server name and permission-aware capabilities: [API-FOUNDATIONS.md](API-FOUNDATIONS.md).
  The web settings field is already implemented; native apps can show the returned name.
- Unmatch with preserved files/user references: [UNMATCH-API.md](UNMATCH-API.md).
- Persistent profile/device queues and revision conflicts: [PLAY-QUEUES-API.md](PLAY-QUEUES-API.md).
  Queue mutations do not themselves start or stop playback.
- General SSE invalidation/replay and background producers: [EVENTS-API.md](EVENTS-API.md).
  These are hints to refetch, not durable record patches.
- Watch Together rooms, profile invitations and their separate snapshot stream: [WATCH-TOGETHER-API.md](WATCH-TOGETHER-API.md).
- Provider-ID/batch lookup: [ITEM-LOOKUP-API.md](ITEM-LOOKUP-API.md).
- Filters/facets: [FILTER-DISCOVERY-API.md](FILTER-DISCOVERY-API.md).
- Recommendation continuation: [RECOMMENDATION-PAGES-API.md](RECOMMENDATION-PAGES-API.md).
- Read-only decisions/native capabilities: [PLAYBACK-PREVIEW-API.md](PLAYBACK-PREVIEW-API.md).
- Regional artwork colors: [ARTWORK-PALETTES.md](ARTWORK-PALETTES.md).
- Artwork/collection adapters: [AUTOMATION.md](AUTOMATION.md).

JSON errors retain error/needs/domain fields and add code/request_id; correlation matches
X-Request-ID. Cross-origin Bearer SSE supports Last-Event-ID. Older servers and plain-text errors
may omit these fields. See the walkthrough for retry and compatibility rules.

## Pending integration — 11 November 2026 checkpoint

- Review integration [#418](https://github.com/arc-media-server/arc-media-server/pull/418) combines
  the #350 server review fixes. It remains a draft awaiting owner/Claude review; it is not main or
  deployment acceptance. Feature guides label the pending DV-base and per-profile SSE behavior.
- PostgreSQL [#337](https://github.com/arc-media-server/arc-media-server/pull/337) and
  [#339](https://github.com/arc-media-server/arc-media-server/pull/339) remain separate pending work
  under #308 and require compatibility work with the review integration. SQLite remains the
  implemented main backend. Do not expose pending PostgreSQL UI as deployed functionality.
- Offline downloads/sync (#62) are separate planning work.

The original implementation handoffs are historical evidence in dated docs/changes notes. Completed
server-name, queue and background-event work is not a new frontend to-do list. Reassess this dated
pending section when those PRs merge; continue detecting actual server capabilities at runtime.

Tests use temporary stores, generated clips and local providers. Physical clients/receivers,
HDR/audio output, deployment and sustained-use acceptance are separate from API fixture evidence.

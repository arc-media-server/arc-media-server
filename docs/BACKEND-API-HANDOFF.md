# Backend API handoff for clients

Start with [CLIENT-INTEGRATION.md](CLIENT-INTEGRATION.md). The generated [API reference](API.md)
and [OpenAPI contract](openapi.json) describe the current server.
Use `/api/v1`, detect capabilities on connection/profile changes, and keep UI changes in the
existing shared controls. The bundled `/docs/` viewer already exists.

## Implemented client contracts

These features are implemented, rather than proposals for another backend batch:

- Identity, server name and permission-aware capabilities: [API-FOUNDATIONS.md](API-FOUNDATIONS.md).
- Unmatch: [UNMATCH-API.md](UNMATCH-API.md). Removes provider metadata/overrides/orderings while
  preserving files and user references; explicit Fix Match re-enables automatic matching.
- Persistent profile/device-owned queues with revision checks: [PLAY-QUEUES-API.md](PLAY-QUEUES-API.md).
  Playback start/stop remains a client action, separate from changing queue contents.
- General live invalidation/replay: [EVENTS-API.md](EVENTS-API.md). Refetch on hints/resync;
  this is not a record patch feed or guaranteed delivery.
- Provider-ID and batch lookup: [ITEM-LOOKUP-API.md](ITEM-LOOKUP-API.md).
- Rich filter discovery: [FILTER-DISCOVERY-API.md](FILTER-DISCOVERY-API.md).
- Recommendation continuation: [RECOMMENDATION-PAGES-API.md](RECOMMENDATION-PAGES-API.md).
- Read-only playback decisions and native capabilities: [PLAYBACK-PREVIEW-API.md](PLAYBACK-PREVIEW-API.md).
- Regional artwork colors: [ARTWORK-PALETTES.md](ARTWORK-PALETTES.md).

## This follow-up and pending integration

This API-polish branch adds cross-origin SSE cursor support, server-issued error correlation,
logical query constraints and media/header/SSE documentation.
JSON errors retain `error`, `needs` and domain fields, adding `code`/`request_id`; success bodies
are unchanged. See the integration guide for compatibility with older servers and non-JSON errors.

At the review checkpoint, these separate PRs remain dependencies or independent work. Their
presence in a handoff is not evidence that a deployed server has them; confirm GitHub merge state
and the server's capabilities before exposing controls:

- #333: Watch Together rooms and their separate snapshot event protocol.
- #334: conversion concurrency/throttling reliability.
- #335: more precise fixed-map JSON response schemas.
- #336: additional autonomous background-worker invalidation producers.

PostgreSQL support (#337, #339) is parked (#308); SQLite is Arc's database. Offline downloads/sync (#62) are also separate; no client API parity claim
is made. Test evidence uses temporary stores/local fixtures, not owner media or production systems.

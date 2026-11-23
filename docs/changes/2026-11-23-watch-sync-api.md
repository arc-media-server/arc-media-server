# Watch state and history sync API

External tools can set or clear a profile's saved resume position without
reporting player activity. They can also page completed viewings for visible
items and add or remove individual viewings with their original timestamps.

- `PUT /api/v1/items/{id}/resume` changes the position alone.
- `GET /api/v1/watch-history` pages completed entries by viewing ID.
- `POST /api/v1/items/{id}/watch-history` deduplicates by profile, item and Unix
  timestamp in one write transaction. Different dates retain separate viewings.
- `DELETE /api/v1/items/{id}/watch-history/{viewing}?watched_at=...` removes the
  matching entry from the signed-in profile. The item and date protect against
  a stale delete matching a reused SQLite ID.

Dated history and current watched status remain independent. Existing watch
status, rating and watchlist APIs still apply. The new routes don't create
playback sessions or send playback webhooks. Successful writes use the usual
private profile invalidation. Request failures use the existing diagnostics
without logging request data. No migration is needed.

See [the client integration guide](../CLIENT-INTEGRATION.md#sync-watched-state-resume-points-and-dated-history)
for the sync order and wire details. This change adds Arc's API surface.

## Validation

The new API regression passed under the race detector on SQLite and on a
disposable PostgreSQL 19 instance. It checks concurrent retries, profile
isolation, access filtering, pagination, deletion and independent resume state.
Vet, API documentation generation, web typecheck, Vite build and embedded
server build passed. The full Go suite had one existing download revocation
test failure that passed a focused race rerun. No live CrossWatch connector
or owner deployment was tested.

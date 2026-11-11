# Recommendation continuation

Rows from `GET /api/v1/libraries/{id}/recommended` now include `id`, `items_url` and `has_more`.
Keep existing row titles/types and initial 20 cards. When scrolling further, GET `items_url` with
`limit` (1–100, default 50) and `offset` (number already shown, maximum 10000000).
Response: `items`, `total`, `offset`, `has_more`, `bounded`. Append cards; stop when has_more is false.

The continuation uses the same query, watch state, library permissions and row layout as its source.
Ordinary rows include the full matching library. Curated "Because you watched" rows draw from at
most 500 similarity candidates, then remove completed titles. Continue Watching retains the
existing 30 in-progress / 40 recently-watched-series windows. Those responses set `bounded:true`:
total describes that selection, not every possible recommendation. Initial personalized has_more
can be conservative; an empty continuation is valid.

IDs identify the row type and source parameters, not a snapshot. A changed daily selection,
renamed source, hidden/removed row or different current recommendations may return 404: refresh
the parent endpoint. Concurrent additions/watch changes can shift offsets; deduplicate item IDs on
append and refresh on focus. No snapshot guarantee or Plex internal-algorithm equivalence is claimed.
No provider requests, probes or jobs are started. Clients decide how to expose continuation; the endpoint does not change the saved row layout.

Logging reuses existing bounded request failures/debug records; no per-card info output.
Tests use generated records/files and check >20 continuation, totals, no duplicates in a stable
library, permission boundaries, invalid bounds and hidden rows. Real client integration remains.

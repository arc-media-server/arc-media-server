# Exact lookup and batch reads — Claude/integration handoff

Authenticated profile routes under `/api/v1`:

- `GET /items/lookup?provider=tmdb&provider_id=603&kind=movie` returns a normal items/total/offset
  page. Also supports tvdb and IMDb tt identifiers (case-insensitive). Numeric provider IDs must be
  positive. Optional library_id, kind, limit (1–100, default 50), offset (0–10000000).
- `POST /items/batch {"ids":["<Arc ID>","<Arc ID>"]}` returns `{"items":[...]}` with the same
  detail fields as individual GET, including files/streams/watch data. Limit 100 input IDs/16 KiB.
  Preserves requested order and duplicates; missing and restricted IDs are omitted identically.

Lookup returns **all permitted matches**, not one guessed library/edition. Results are ID-sorted,
access-filtered before counting/paging. Exact lookup/batch can return known offline identities:
these APIs manage records and do not claim present media. Neither starts providers, scanning, probes
or changes state. An empty lookup page does not reveal whether a hidden match exists. A client may
page matches or narrow library/kind explicitly. Use Arc IDs for mutations, not provider IDs.

Invalid input returns 400, unsigned/profile-unselected returns 401; failures retain existing error
format. Replies are private/no-store. Use capability flags `item_lookup` and `item_batch`.
No success/poll/per-item logging was added; failures reuse the API boundary. Three provider lookup
indexes avoid full-library client downloads. No UI work is required for automation; Claude can use
these routes in app adapters. This is not a Plex/Kometa compatibility shim or IMDb ratings ingestion.

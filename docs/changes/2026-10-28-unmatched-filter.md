# Unmatched filter

The owner asked for Plex's Unmatched library filter.

- The library's Filters panel has "Only titles Arc couldn't identify (n)" for the server owner,
  in movie and TV libraries (not Other videos, which are never looked up). The count says how
  many it finds; when it's on, an "Unmatched" chip removes it, as for the other filters.
- In TV libraries it also finds shows Arc identified but with an episode on disk it couldn't
  place (an S01E20 the provider doesn't have), the same things Library Health lists as not
  identified. Plex's filter finds only unidentified shows; the difference is logged on #80 and
  easy to narrow.
- API: `GET /api/items?unmatched=1`; `GET /api/libraries/{id}/facets` adds `unmatched`, the count.

Checks: `TestUnmatchedFilter` (an unplaced episode without a file and identified titles are left
out) and the browser journey "Unmatched filter" (movies, a show with an unplaced episode, and a
profile without admin rights not offered the switch).

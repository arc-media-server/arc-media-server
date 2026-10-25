# Library letter navigation (#188)

The owner selected jump-to-letter on 25 October 2026, replacing the earlier deferral.
The accepted task is reaching a known alphabetical section without scrolling through
all earlier posters. Density and list views remain outside this change.

## User contract

- Movies, shows and other-video title grids offer **Jump to → All titles / A–Z**
  when sorted by Title. Choosing a letter starts there, or at the next available
  title in the current direction. Reverse starts at the end of the chosen letter.
- This is a starting position, not a letter-only filter. Load more continues through
  subsequent letters. All titles resets the position while retaining filters/direction.
  Switching to another sort clears the letter; Collections/Recommended have no selector.
- Existing filters, playable-file checks, age/library/label restrictions and the full
  filtered title count remain. Jump offsets are calculated within that same query,
  including titles beyond the first 500, rather than downloading all preceding pages.
- A letter with no remaining results explains the state and offers All titles. Request
  failures retain normal Retry. Bulk selection continues to mean only loaded titles.
- The letter is in the URL, not a new profile setting. Saved default sort behavior stays.
  Returning from a title restores loaded pages from the existing query cache and the
  history entry's scroll position. Position storage is bounded, profile-scoped, in-memory
  in the current tab; it is not a cross-device or durable scroll bookmark.
- Boundaries follow existing saved sort titles, including English article removal and
  custom sort-title edits. No new locale collation or transliteration is introduced.
  Numeric, punctuation, accented and non-Latin titles remain accessible through All
  titles and normal paging; the shortcut itself offers the requested A–Z letters.

## API

`GET /api/items?library_id=…&kind=movie&sort=title&jump=S&limit=500` returns
`items`, the full filtered `total`, and its actual `offset`. The first page counts
matching rows preceding the requested boundary using the same WHERE/access conditions.
Following pages omit `jump` and use `offset + items.length`. A jump overrides the
initial supplied offset. Invalid letters or combining a jump with another sort return
400. Existing callers of ListItems retain the same signature and ordinary behavior.
No schema, dependency, scanner, provider, source-media or watch-history changes.
As with existing offset paging, concurrent library changes can shift later offsets;
this does not introduce a snapshot of the library.

## Comparison and verification boundary

Reference from the audit: [Plex library view](https://support.plex.tv/articles/200392126-using-the-library-view/).
A fresh fetch during this implementation returned HTTP 403 and search returned no
result; exact current Plex shortcut behavior was not verified or claimed as parity.
Arc's scope follows the owner's explicit choice and existing title-order behavior.

Actual checks and limitations are recorded in READINESS and GitHub #188/#162. Synthetic
large grids prove navigation/request/paging behavior, not scanner speed, memory scaling,
physical-device behavior or benefit measured on the owner's real collection.

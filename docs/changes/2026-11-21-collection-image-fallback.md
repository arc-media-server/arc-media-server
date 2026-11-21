# A collection's image link falls back to its first title's

**What:** `GET /api/v1/collections/{id}/images/{primary|backdrop}` now answers with the
collection's first title's picture when the collection has none of its own, instead of 404 "no
image". An empty collection still answers 404.

**Why:** a title's `collections[]` carries no images, so apps link straight to this route; for a
collection without its own artwork the Kodi title page's Collections row showed blank posters
(owner, 21 November). The collection listings already fall back to the first title's artwork
(`collectionDTO`); the route now agrees with them.

**API:** same route, statuses and fields; no contract change.

**Checks:** new `TestCollectionImageFallsBackToFirstTitle` (fails without the change), `go test
./internal/api/`, `go vet ./...`.

**Logging:** nothing new; failures go through the existing `s.fail`.

## Imported collections get their TMDB collection's artwork

**What:** a manual collection (made by hand, or imported from Plex or Jellyfin, which arrive as
manual collections with no TMDB link) whose titles share a TMDB collection (more than half of
them, at least two) now gets that collection's poster and background, as an automatic collection
does, and its artwork editor lists that collection's TMDB posters and backgrounds instead of
"TMDB has no poster images". Artwork already there or chosen by hand is never replaced; the
collection's own title and description are left alone.

**When:** on opening the collection, and in the background when a library's collections are
listed (so the listing isn't held up by TMDB; the next listing shows the artwork). Each lookup is
backed off like the automatic collections' description lookup (a day after success, a minute
after failure).

**Why:** the owner's Plex-imported collections all showed their first film's poster and
background, and the editor offered nothing.

**Decisions:** a collection's titles must mostly share one TMDB collection, so a mixed list (a
"100% FEL" collection) keeps its first title's artwork rather than borrowing one film series'.
Plex keeps a collection's imported artwork; Arc's Plex import doesn't bring artwork, so this is
the nearest equivalent.

**Checks:** `TestManualCollectionUsesSharedTMDBCollectionArtwork` (a TMDB fixture: artwork filled
on open, editor lists the images, hand-chosen artwork kept, a mixed collection gets none),
`go test ./internal/api/ ./internal/store/`, `go vet ./...`.

**Logging:** a failed lookup or save is a warning once per collection (no key or URL); the
background pass logs one info line with how many collections it filled, how many it checked and
its duration.

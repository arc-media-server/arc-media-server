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

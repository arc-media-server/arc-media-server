# Imported collections get TMDB's description too

**What changed:** an imported (manual) collection whose titles mostly share one TMDB collection now
also gets that collection's description from TMDB, not just its poster and background
(`docs/changes/2026-11-21-collection-image-fallback.md`). As before, only empty fields are filled:
a description written or locked by hand is never replaced. `FillCollectionArtwork` became
`FillCollectionInfo`.

**Why:** collections imported from another server arrive without a description, while Arc's own
automatic collections already show TMDB's.

**Checks:** `go vet`, `go test ./...`; the shared-artwork test now also covers a collection with no
description getting TMDB's, and a hand-written one being kept.

**Logging:** the background fill's info line is now "collection details filled from TMDB", with
the same counts and duration; failures warn once per collection as before.

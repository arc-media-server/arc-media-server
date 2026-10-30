# The items page joins the v1 contract

**What changed.** `GET /items` answered with an untyped map, so the v1 contract listed the
endpoint but none of its fields. It now answers with a typed `itemPage` (`total`, `items`,
`offset`, and `letters` with `letters=1`), and the contract promises them. With `letters=1` and
nothing to count, `letters` is now `[]` instead of `null`.

**Why.** The Android TV app's library gets sorting, filters and an A–Z jump bar
(arc-android#1), which rely on `total`, `offset` and `letters`.

**Checks.** `go vet`, `go test` for `internal/api` and `internal/apidoc`; `go run ./tools/apidoc`
added the contract lines (additive only).

**Logging.** Nothing new.

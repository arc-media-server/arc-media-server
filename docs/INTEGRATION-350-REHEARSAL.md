# #350 integration rehearsal and review handoff

This branch combines the 67 pinned review PRs #351–#417 on main `d94283c`. It is a rehearsal for review, not permission to merge or publish a development image. Main and original PR branches remain unchanged. PostgreSQL PRs #337/#339 are separate and excluded.

## Reproduce the composition

Each original head is retained as an ancestor through a local merge commit, in the order below. There were 29 conflicting merges across 20 files. Their resolutions are part of the corresponding merge commits; later test/documentation adjustments are separate. Replaying only the original PR heads on main does not reproduce the resolved tree automatically.

| PR | Pinned head | Merge outcome |
|---|---|---|
| #351 | `c3514fcaf785` | Clean |
| #352 | `19604c5ad246` | Clean |
| #353 | `a00dd498df98` | Clean |
| #354 | `2ea222d40784` | Clean |
| #355 | `15643ef6c2dd` | Clean |
| #356 | `305930cf72b5` | Clean |
| #357 | `cc0d26009407` | Clean |
| #358 | `494dae850940` | Clean |
| #359 | `75a54530a7dc` | Clean |
| #360 | `871922366413` | Clean |
| #361 | `60aad54ceb7c` | Resolved: `internal/scanner/match_fixture_test.go` |
| #362 | `82ce87337121` | Clean |
| #363 | `3c8cc271a525` | Clean |
| #364 | `2a0575eb7eb3` | Clean |
| #365 | `d12b43f0f943` | Clean |
| #366 | `d09ad8023c16` | Clean |
| #367 | `1cac48c33f05` | Clean |
| #368 | `1000e0b23ebf` | Clean |
| #369 | `8c6ca9c90ac8` | Clean |
| #370 | `e8186e43ca96` | Resolved: `internal/transcode/transcode_test.go` |
| #371 | `038123dd2687` | Clean |
| #372 | `0de7b6cb8c48` | Resolved: `docs/CLIENT-INTEGRATION.md`, `internal/api/playback.go`, `internal/transcode/transcode.go`, `web/e2e/journeys.spec.ts` |
| #373 | `2baba2e44d47` | Resolved: `internal/notify/notify.go` |
| #374 | `d85299798cce` | Clean |
| #375 | `e87f635003fc` | Clean |
| #376 | `8268e6d4db70` | Clean |
| #377 | `5fe001c1fb3d` | Resolved: `docs/CLIENT-INTEGRATION.md`, `internal/api/bulk.go` |
| #378 | `ebe132572643` | Resolved: `docs/CLIENT-INTEGRATION.md` |
| #379 | `a5f3108473eb` | Clean |
| #380 | `20204af1aa40` | Resolved: `internal/api/friends.go` |
| #381 | `33a084bce9cc` | Resolved: `docs/CLIENT-INTEGRATION.md` |
| #382 | `ebdc35337490` | Clean |
| #383 | `6c92bbfd02ce` | Clean |
| #384 | `271aacfe86c5` | Resolved: `docs/CLIENT-INTEGRATION.md` |
| #385 | `c83954692396` | Resolved: `docs/CLIENT-INTEGRATION.md` |
| #386 | `8d2fd43da491` | Resolved: `docs/CLIENT-INTEGRATION.md` |
| #387 | `5438f0970575` | Resolved: `internal/api/playback.go` |
| #388 | `3e50a37f0972` | Resolved: `docs/CLIENT-INTEGRATION.md` |
| #389 | `db71931286b5` | Resolved: `docs/CLIENT-INTEGRATION.md`, `internal/api/libraries.go` |
| #390 | `f6021968cb30` | Clean |
| #391 | `cc6623470975` | Resolved: `internal/scanner/scanner.go` |
| #392 | `c050106471ff` | Resolved: `internal/transcode/transcode.go`, `internal/transcode/transcode_test.go` |
| #393 | `05dc0a210fdc` | Clean |
| #394 | `61ea09094169` | Clean |
| #395 | `4b06617a9c19` | Resolved: `internal/api/api.go` |
| #396 | `72ce6d6a2479` | Resolved: `docs/openapi.json` |
| #397 | `bfadbdaf0d5c` | Resolved: `internal/transcode/transcode.go` |
| #398 | `eea16f32aed4` | Resolved: `internal/api/api.go`, `internal/api/imports.go` |
| #399 | `3507e0018dd2` | Clean |
| #400 | `83e4c78894e6` | Clean |
| #401 | `8db4cc3fe27d` | Clean |
| #402 | `949502ab9390` | Resolved: `internal/api/collections.go` |
| #403 | `7e4ed67ad20f` | Clean |
| #404 | `d035885cd7b1` | Clean |
| #405 | `f41053a79165` | Clean |
| #406 | `e15d60f0225e` | Resolved: `internal/api/playlists.go` |
| #407 | `26ad7b9005a8` | Clean |
| #408 | `38e694a75b62` | Resolved: `internal/store/libraries.go`, `internal/store/library_settings.go` |
| #409 | `0f42279ad6da` | Clean |
| #410 | `9eda25bb4fa4` | Resolved: `cmd/ams/main.go` |
| #411 | `e2990b0d7ea1` | Resolved: `internal/notify/notify.go` |
| #412 | `bf3cfaeb628d` | Resolved: `cmd/ams/main.go`, `docs/openapi.json`, `internal/api/api.go`, `internal/api/bulk.go`, `internal/api/imports.go`, `internal/notify/notify.go` |
| #413 | `1358ee07b247` | Resolved: `internal/api/api.go`, `internal/api/playback.go` |
| #414 | `c62bb02ef6cf` | Clean |
| #415 | `db3c777e4fa2` | Resolved: `internal/api/playlists.go` |
| #416 | `53bdf07f281e` | Resolved: `internal/api/review_hot_paths_test.go` |
| #417 | `cac01600306b` | Clean |

## Resolutions that need reviewer attention

- #354 + #380: retain atomic credential admission and constant-time unknown-friend work, without a second limiter failure count.
- #366/#367/#368/#370/#371/#372/#374/#387/#392: retain AAC repackaging, source-relative offsets, compatible DV-base selection, storage/cancellation checks, bounded stderr and slow-start activity together. Fit scaling precedes tone mapping; PGS composition uses the same scaled video.
- #365/#375/#389/#403/#408: keep catalog/scanner admission and transactional library saves while retaining typed validation errors and preparing root relocation outside the writer transaction.
- #365/#377/#398/#407/#412: keep the richer bounded bulk queue, atomic synchronous edits, import admission/session retention and root worker ownership. Remove the older duplicate import catalog-admission wrapper. Process shutdown joins requests before the asynchronous API group and closes dependencies afterward.
- #356/#373/#411/#412: retain safe webhook retries, attempt/duration diagnostics and root cancellation/joining together.
- #384/#389/#406/#415: preserve typed smart-filter validation and the shared filter-to-query conversion while selecting narrow playlist summaries.
- #415/#416: retain both independent allocation regression tests and one shared fixture.
- #383/#417: retain one header/CSP implementation and current browser expectations.
- Generated OpenAPI conflicts are resolved by regenerating from combined handlers. The original main v1 contract has no missing lines. Client guidance now describes #407 atomic bulk edits rather than the older pre-#407 behavior.

## Actual attempts and results

- Initial compile check failed: two test ApplySettings callbacks used the old signature. Updated those fixture callbacks to receive before/next.
- Initial full Go run: 35 package passes, 1 package failed (API), 7 packages have no tests. The failing restore assertion counted a separate mandatory scan-resumption diagnostic as a duplicate request error. A focused red run confirmed one request ERROR and one recovery WARN. The test now checks those exact messages/levels for restore and still requires a single request error elsewhere; production diagnostics are retained.
- Fresh `go vet ./...` and full Go suite passed: 36 packages, 0 failures, 7 packages have no tests.
- Production web build, bundled translation/unit checks and app/e2e TypeScript passed.
- Opt-in allocation regressions passed on the combined code: playlist summaries 19.498 MiB (32 MiB bound), curated recommendation page 3.625 MiB (16 MiB bound). These are memory envelopes, not production latency promises.
- Real-server malformed-input sweep passed: 406 requests, 0 failed, 7 skipped operations (14 alias requests for bodyless commands/live streams), 0 provider calls. Two expected missing-configuration 503s remain classified explicitly. The first selector matched no tests; that command is retained separately and not counted.
- Container lifecycle checks passed for the default UID10001, UID0 and UID12345: first-run setup, read-only media, backup/restore, restart and volume persistence. Temporary containers/volumes are cleaned up by the test. Runtime-overlay limitations are below.
- First full integrated browser run: **165 passed, 2 failed, 0 skipped/retried (167 journeys)**. The trusted-proxy test assumed an arbitrary RFC1918 address belonged to the host's detected LAN; configure the synthetic LAN explicitly while keeping untrusted/public/forged-chain assertions. A Cast fallback counter also included an unfinished initial local plan; wait for that local response before counting the exact receiver original/fallback requests. No product behavior or security rule was changed. Failed JSON/logs/traces remain retained.
- Both corrected browser controls passed together (2/2); e2e TypeScript passed after those edits.
- Whole-server race and final fresh full 167-journey pinned-browser run: PENDING completion; enter actual results before opening the review PR.

## Scope and limits

Only temporary databases, generated media, local provider fixtures and disposable container volumes. No owner data, keys, Unraid or client repositories. Every shell uses the prepared Go 1.27 and Jellyfin FFmpeg 8.1.3 activation; browser is the prepared Chromium build 1248/Chrome156, no substitute or new install.

Container validation uses the already-tested pinned/nonroot runtime image with the freshly compiled combined embedded binary copied into it, without network access. It is not a clean multi-stage Dockerfile build, multi-architecture or physical GPU/NAS acceptance. Native HDR/DV output, physical receivers, sustained usage and production deployment remain separate.

PostgreSQL compatibility is not established here. Its PRs #337/#339 require their own integration with the additional #350 SQLite migrations/query/lifecycle changes and tested baseline/upgrade updates. Do not imply that merging the PostgreSQL PRs after this branch is automatically compatible.

## Logging

No new product logging is introduced for the rehearsal. Existing per-operation diagnostics remain; test assertions distinguish the request failure from failure of a separate required recovery action. No keys, tokens, raw URLs, request bodies or SQL arguments are added to logs.

Review this resolved composition before merging anything. The owner has not authorized a main merge or deployment.

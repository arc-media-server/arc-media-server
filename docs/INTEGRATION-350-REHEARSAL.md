# #350 integration rehearsal and review handoff

This branch combines the 67 pinned review PRs #351–#417 on main `2d8d4ea`. It is a rehearsal for review, not permission to merge or publish a development image. Main and original PR branches remain unchanged. PostgreSQL PRs #337/#339 are separate and excluded.

## Reproduce the composition

Each original head is retained as an ancestor through a local merge commit, in the order below. There were 29 conflicting merges across 20 files. Their resolutions are part of the corresponding merge commits; later test/documentation adjustments are separate. Replaying only the original PR heads on main does not reproduce the resolved tree automatically.

| PR | Pinned head | Rehearsal merge | Merge outcome |
|---|---|---|---|
| [#351](https://github.com/arc-media-server/arc-media-server/pull/351) | `b43fb86b2212` | [61e091e7880e](https://github.com/arc-media-server/arc-media-server/commit/61e091e7880e2523360577074e20f5f4c7a0de74) | Clean |
| [#352](https://github.com/arc-media-server/arc-media-server/pull/352) | `ac719c490010` | [232fe49ebbe9](https://github.com/arc-media-server/arc-media-server/commit/232fe49ebbe97e1adebbb2432f747568da8b5771) | Clean |
| [#353](https://github.com/arc-media-server/arc-media-server/pull/353) | `c7b445b55ee7` | [15c56a798789](https://github.com/arc-media-server/arc-media-server/commit/15c56a798789ebc60154527feccd615b148acb79) | Clean |
| [#354](https://github.com/arc-media-server/arc-media-server/pull/354) | `06e514dd98fc` | [869827274847](https://github.com/arc-media-server/arc-media-server/commit/8698272748476e6ad4977432fb120eb192948ec2) | Clean |
| [#355](https://github.com/arc-media-server/arc-media-server/pull/355) | `dba7a593598d` | [77552c0c1f57](https://github.com/arc-media-server/arc-media-server/commit/77552c0c1f5792a0c40519ddcd351bb671d44c29) | Clean |
| [#356](https://github.com/arc-media-server/arc-media-server/pull/356) | `5a438b805977` | [6b14135abbfc](https://github.com/arc-media-server/arc-media-server/commit/6b14135abbfc27e992c38eefd377ed5ca6290e3b) | Clean |
| [#357](https://github.com/arc-media-server/arc-media-server/pull/357) | `23b693d49ed8` | [de54b2778a11](https://github.com/arc-media-server/arc-media-server/commit/de54b2778a1110aa357e558001587d18277d8034) | Clean |
| [#358](https://github.com/arc-media-server/arc-media-server/pull/358) | `4cf2b3d43c40` | [97c611189e9f](https://github.com/arc-media-server/arc-media-server/commit/97c611189e9f6075c2d83d3df22ab3e7900e4045) | Clean |
| [#359](https://github.com/arc-media-server/arc-media-server/pull/359) | `40c85d0bfdca` | [70fbf5f6a21c](https://github.com/arc-media-server/arc-media-server/commit/70fbf5f6a21c22e24e8282a8425ff8f063b1809b) | Clean |
| [#360](https://github.com/arc-media-server/arc-media-server/pull/360) | `ec9674ad1c29` | [2876b3a4d6ea](https://github.com/arc-media-server/arc-media-server/commit/2876b3a4d6eac59a82be3c869fdccbacd0f3215e) | Clean |
| [#361](https://github.com/arc-media-server/arc-media-server/pull/361) | `c9388f522cad` | [a62b1f4de99b](https://github.com/arc-media-server/arc-media-server/commit/a62b1f4de99b3d889433183292a96c31b9348c9d) | Resolved: `internal/scanner/match_fixture_test.go` |
| [#362](https://github.com/arc-media-server/arc-media-server/pull/362) | `344d32b65aef` | [2a6d3e978544](https://github.com/arc-media-server/arc-media-server/commit/2a6d3e978544e41c7fbf3545eadd001bc2f16d64) | Clean |
| [#363](https://github.com/arc-media-server/arc-media-server/pull/363) | `18c3761a603b` | [7a82f3c82396](https://github.com/arc-media-server/arc-media-server/commit/7a82f3c82396456c5ebbc8cb66c20521566697fc) | Clean |
| [#364](https://github.com/arc-media-server/arc-media-server/pull/364) | `93210ad549a3` | [f686d7f8b01f](https://github.com/arc-media-server/arc-media-server/commit/f686d7f8b01f6cf6d15a169a24c1981ae92ef1e9) | Clean |
| [#365](https://github.com/arc-media-server/arc-media-server/pull/365) | `ffe2bd6c407c` | [9fe43a86fb07](https://github.com/arc-media-server/arc-media-server/commit/9fe43a86fb0787ae10640d115975c5c0eda6a3be) | Clean |
| [#366](https://github.com/arc-media-server/arc-media-server/pull/366) | `0cdc9d2cd62d` | [8f60f0b518df](https://github.com/arc-media-server/arc-media-server/commit/8f60f0b518dfe05c0c0b59631b74790747d2c4ef) | Clean |
| [#367](https://github.com/arc-media-server/arc-media-server/pull/367) | `358690b27eaf` | [47b97a6eaf9f](https://github.com/arc-media-server/arc-media-server/commit/47b97a6eaf9f3b8781c4f2174ccd9144d4d70f5e) | Clean |
| [#368](https://github.com/arc-media-server/arc-media-server/pull/368) | `f5b8d00ebe7a` | [ac25f38b8bda](https://github.com/arc-media-server/arc-media-server/commit/ac25f38b8bda1635954f68ed3c28b4341724e0b5) | Clean |
| [#369](https://github.com/arc-media-server/arc-media-server/pull/369) | `924bc1541d53` | [7a1555db9085](https://github.com/arc-media-server/arc-media-server/commit/7a1555db908572100901460585c3ca2214dfbd22) | Clean |
| [#370](https://github.com/arc-media-server/arc-media-server/pull/370) | `620ae153fe58` | [4ef6a86cd204](https://github.com/arc-media-server/arc-media-server/commit/4ef6a86cd20484472388e955eb734034bd8e3432) | Resolved: `internal/transcode/transcode_test.go` |
| [#371](https://github.com/arc-media-server/arc-media-server/pull/371) | `0af2805b2ca7` | [a1ac393796e2](https://github.com/arc-media-server/arc-media-server/commit/a1ac393796e2538dd31d63542772bfdff2aeaad7) | Clean |
| [#372](https://github.com/arc-media-server/arc-media-server/pull/372) | `ed0d7d557f00` | [c75ef1f6adef](https://github.com/arc-media-server/arc-media-server/commit/c75ef1f6adef7d0318fc754188e7dac99f306ee4) | Resolved: `docs/CLIENT-INTEGRATION.md`, `internal/api/playback.go`, `internal/transcode/transcode.go`, `web/e2e/journeys.spec.ts` |
| [#373](https://github.com/arc-media-server/arc-media-server/pull/373) | `a82408f1a795` | [41b344143879](https://github.com/arc-media-server/arc-media-server/commit/41b3441438799c9092d86aea1fce2fbb4cc0148d) | Resolved: `internal/notify/notify.go` |
| [#374](https://github.com/arc-media-server/arc-media-server/pull/374) | `a5d85f80244e` | [48cb3e63736f](https://github.com/arc-media-server/arc-media-server/commit/48cb3e63736f581d70ffef570e79ce77003bdfc1) | Clean |
| [#375](https://github.com/arc-media-server/arc-media-server/pull/375) | `7cbd924d8e05` | [7e2a0b2fcdc6](https://github.com/arc-media-server/arc-media-server/commit/7e2a0b2fcdc6617803f3ebfd6bc1652c382e83c2) | Clean |
| [#376](https://github.com/arc-media-server/arc-media-server/pull/376) | `04400435eee4` | [80e4889d79d1](https://github.com/arc-media-server/arc-media-server/commit/80e4889d79d1bca2180903b7b28e88ba2bce68d5) | Clean |
| [#377](https://github.com/arc-media-server/arc-media-server/pull/377) | `d390091ba34f` | [d5aaeef19910](https://github.com/arc-media-server/arc-media-server/commit/d5aaeef199101839d26eb9bc145a51100e9d34da) | Resolved: `docs/CLIENT-INTEGRATION.md`, `internal/api/bulk.go` |
| [#378](https://github.com/arc-media-server/arc-media-server/pull/378) | `22dc4a9c2f9f` | [9961cf6b8c36](https://github.com/arc-media-server/arc-media-server/commit/9961cf6b8c36f1b85955de44162bd864db382544) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#379](https://github.com/arc-media-server/arc-media-server/pull/379) | `4ff67ae4410b` | [a0876af14ff4](https://github.com/arc-media-server/arc-media-server/commit/a0876af14ff474ed36376a6d5de11de7f9e60cd7) | Clean |
| [#380](https://github.com/arc-media-server/arc-media-server/pull/380) | `23d403b7ddcf` | [980358b00d67](https://github.com/arc-media-server/arc-media-server/commit/980358b00d675f0f0d7b95a1c8ec3e68b9640ce5) | Resolved: `internal/api/friends.go` |
| [#381](https://github.com/arc-media-server/arc-media-server/pull/381) | `ddab941e61af` | [2dac0c3d0ab1](https://github.com/arc-media-server/arc-media-server/commit/2dac0c3d0ab1cc19dcf8aa10586015828db103be) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#382](https://github.com/arc-media-server/arc-media-server/pull/382) | `303ba1433edb` | [e84d1d5668bc](https://github.com/arc-media-server/arc-media-server/commit/e84d1d5668bc7e5a3533237de3321219384f540b) | Clean |
| [#383](https://github.com/arc-media-server/arc-media-server/pull/383) | `bdf87c0a52cc` | [cbc8c144c12e](https://github.com/arc-media-server/arc-media-server/commit/cbc8c144c12eed2c0885c6ddd065f234a071c008) | Clean |
| [#384](https://github.com/arc-media-server/arc-media-server/pull/384) | `d329bbbb1500` | [5cc3a65a415d](https://github.com/arc-media-server/arc-media-server/commit/5cc3a65a415d9e747cbc9c3b4a15c4c9488f8f24) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#385](https://github.com/arc-media-server/arc-media-server/pull/385) | `daf203c49771` | [11ae4c58a0d0](https://github.com/arc-media-server/arc-media-server/commit/11ae4c58a0d062e94d6d73ed5643429efd7eb412) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#386](https://github.com/arc-media-server/arc-media-server/pull/386) | `18cad66f1145` | [23d6f534c16f](https://github.com/arc-media-server/arc-media-server/commit/23d6f534c16f7d470ae076b21e5f7b1b73faa2a6) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#387](https://github.com/arc-media-server/arc-media-server/pull/387) | `70e1c49ade3c` | [2b8c6e2096cd](https://github.com/arc-media-server/arc-media-server/commit/2b8c6e2096cd7f22772c020e2eccebe2492d9330) | Resolved: `internal/api/playback.go` |
| [#388](https://github.com/arc-media-server/arc-media-server/pull/388) | `1b90fa116f57` | [7afbd55e0353](https://github.com/arc-media-server/arc-media-server/commit/7afbd55e0353738e919e33edd397419dc62d0860) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#389](https://github.com/arc-media-server/arc-media-server/pull/389) | `41239d701524` | [55db75a97dc6](https://github.com/arc-media-server/arc-media-server/commit/55db75a97dc6e130d11f7b2caac33cec181ce074) | Resolved: `docs/CLIENT-INTEGRATION.md`, `internal/api/libraries.go` |
| [#390](https://github.com/arc-media-server/arc-media-server/pull/390) | `c083fdaaca45` | [08aece732375](https://github.com/arc-media-server/arc-media-server/commit/08aece732375ee93867f9c6c4a538dd386a8eed7) | Clean |
| [#391](https://github.com/arc-media-server/arc-media-server/pull/391) | `b307dfeee6d6` | [d3a68027e3f3](https://github.com/arc-media-server/arc-media-server/commit/d3a68027e3f35e662f69495db302a38fec0d5ab9) | Resolved: `internal/scanner/scanner.go` |
| [#392](https://github.com/arc-media-server/arc-media-server/pull/392) | `226a314ad43f` | [70a9dbfbd14d](https://github.com/arc-media-server/arc-media-server/commit/70a9dbfbd14dad0caf7001cdccbdaeb1fd3f2da3) | Resolved: `internal/transcode/transcode.go`, `internal/transcode/transcode_test.go` |
| [#393](https://github.com/arc-media-server/arc-media-server/pull/393) | `c2f608fecab2` | [6a63b4e30b4c](https://github.com/arc-media-server/arc-media-server/commit/6a63b4e30b4c83321e187e2e0d3b993cbb5c16e6) | Clean |
| [#394](https://github.com/arc-media-server/arc-media-server/pull/394) | `935f0f6f1a87` | [6e0cfc98c50a](https://github.com/arc-media-server/arc-media-server/commit/6e0cfc98c50a02507b33e3bd0be08902a5489804) | Clean |
| [#395](https://github.com/arc-media-server/arc-media-server/pull/395) | `bfd395ba53c4` | [d3f71ce6ed7e](https://github.com/arc-media-server/arc-media-server/commit/d3f71ce6ed7e12b6bc298d330c4a5bf5712bc8f5) | Resolved: `internal/api/api.go` |
| [#396](https://github.com/arc-media-server/arc-media-server/pull/396) | `3cadad99d204` | [0cf17341d252](https://github.com/arc-media-server/arc-media-server/commit/0cf17341d2523f068f2333321554459606b1d507) | Resolved: `docs/openapi.json` |
| [#397](https://github.com/arc-media-server/arc-media-server/pull/397) | `ac7fd39e2465` | [dcea8f5b38ef](https://github.com/arc-media-server/arc-media-server/commit/dcea8f5b38ef4c57f40e93f4cde9a1e95a2270b0) | Resolved: `internal/transcode/transcode.go` |
| [#398](https://github.com/arc-media-server/arc-media-server/pull/398) | `25dd0e29d270` | [ddf2d51338db](https://github.com/arc-media-server/arc-media-server/commit/ddf2d51338dbd0a60821aa661d0870d1dafc714a) | Resolved: `internal/api/api.go`, `internal/api/imports.go` |
| [#399](https://github.com/arc-media-server/arc-media-server/pull/399) | `89263b235336` | [716d7bbd8732](https://github.com/arc-media-server/arc-media-server/commit/716d7bbd873253e3f0e3124850a64a585e6d064b) | Clean |
| [#400](https://github.com/arc-media-server/arc-media-server/pull/400) | `00b804f7a674` | [5df581a7c6c4](https://github.com/arc-media-server/arc-media-server/commit/5df581a7c6c47b70eaefa9e59849209ed3aea6e8) | Clean |
| [#401](https://github.com/arc-media-server/arc-media-server/pull/401) | `63d0229e459d` | [166b2b37b150](https://github.com/arc-media-server/arc-media-server/commit/166b2b37b150fbb69a0af934026374e5777391c2) | Clean |
| [#402](https://github.com/arc-media-server/arc-media-server/pull/402) | `da2819473799` | [76096b4ff1e8](https://github.com/arc-media-server/arc-media-server/commit/76096b4ff1e823ad0f117db33dc6171541045a56) | Resolved: `internal/api/collections.go` |
| [#403](https://github.com/arc-media-server/arc-media-server/pull/403) | `6a2e175e81dd` | [fa0c41a683e2](https://github.com/arc-media-server/arc-media-server/commit/fa0c41a683e23107552270b869c7d7e41355f9b4) | Clean |
| [#404](https://github.com/arc-media-server/arc-media-server/pull/404) | `776a02ef9b43` | [8aec1b8ae49b](https://github.com/arc-media-server/arc-media-server/commit/8aec1b8ae49bb9a7825ee6f2e84a9e47f68c6d0f) | Clean |
| [#405](https://github.com/arc-media-server/arc-media-server/pull/405) | `63b21893eba5` | [ca73bbaac137](https://github.com/arc-media-server/arc-media-server/commit/ca73bbaac1371c4e5e03f9ee31981880b65f89dd) | Clean |
| [#406](https://github.com/arc-media-server/arc-media-server/pull/406) | `e74df5ddccc3` | [b0642ef176dc](https://github.com/arc-media-server/arc-media-server/commit/b0642ef176dc632486518e46e0e7776ed9148968) | Resolved: `internal/api/playlists.go` |
| [#407](https://github.com/arc-media-server/arc-media-server/pull/407) | `b6d8a576052b` | [069c42932cde](https://github.com/arc-media-server/arc-media-server/commit/069c42932cdede558e922766247d14ab690dc345) | Clean |
| [#408](https://github.com/arc-media-server/arc-media-server/pull/408) | `af58d03aaff4` | [584d6b9f701d](https://github.com/arc-media-server/arc-media-server/commit/584d6b9f701d5fcc6c668788dfef620e3d57d86c) | Resolved: `internal/store/libraries.go`, `internal/store/library_settings.go` |
| [#409](https://github.com/arc-media-server/arc-media-server/pull/409) | `2ca8d570d11e` | [ddbae6a1d78c](https://github.com/arc-media-server/arc-media-server/commit/ddbae6a1d78c63d68e6f489de0e5e524ccfead0e) | Clean |
| [#410](https://github.com/arc-media-server/arc-media-server/pull/410) | `828320b7efca` | [9f38e15459f0](https://github.com/arc-media-server/arc-media-server/commit/9f38e15459f04e811cf93503d1b0624d857d3491) | Resolved: `cmd/ams/main.go` |
| [#411](https://github.com/arc-media-server/arc-media-server/pull/411) | `117acdc9effd` | [e0478e369bb4](https://github.com/arc-media-server/arc-media-server/commit/e0478e369bb4f12df43fd68c71b22336ec1b952c) | Resolved: `internal/notify/notify.go` |
| [#412](https://github.com/arc-media-server/arc-media-server/pull/412) | `d36c9b6d2443` | [154db243aff3](https://github.com/arc-media-server/arc-media-server/commit/154db243aff319cacf42a0a54d23b6fac18b6831) | Resolved: `cmd/ams/main.go`, `docs/openapi.json`, `internal/api/api.go`, `internal/api/bulk.go`, `internal/api/imports.go`, `internal/notify/notify.go` |
| [#413](https://github.com/arc-media-server/arc-media-server/pull/413) | `404847456128` | [a78e7cee5f83](https://github.com/arc-media-server/arc-media-server/commit/a78e7cee5f83edee0e75d72d2bba9ecc42a1620a) | Resolved: `internal/api/api.go`, `internal/api/playback.go` |
| [#414](https://github.com/arc-media-server/arc-media-server/pull/414) | `3a7ebc703a1d` | [ac59cc1e9fe8](https://github.com/arc-media-server/arc-media-server/commit/ac59cc1e9fe878a48576de0af4c484f4126664e3) | Clean |
| [#415](https://github.com/arc-media-server/arc-media-server/pull/415) | `382ccb137fc6` | [ca2f22eee520](https://github.com/arc-media-server/arc-media-server/commit/ca2f22eee520b6d74bd12928c369b03184fdeca4) | Resolved: `internal/api/playlists.go` |
| [#416](https://github.com/arc-media-server/arc-media-server/pull/416) | `94ba025fef2f` | [5b6a6bed41a7](https://github.com/arc-media-server/arc-media-server/commit/5b6a6bed41a71b4c2073cc768feb19480a8c0aeb) | Resolved: `internal/api/review_hot_paths_test.go` |
| [#417](https://github.com/arc-media-server/arc-media-server/pull/417) | `4b6e8befcf42` | [5c302b457cae](https://github.com/arc-media-server/arc-media-server/commit/5c302b457cae7d5ec77f8085405b3f7c32999a8f) | Clean |

## Resolutions that need reviewer attention

- #354 + #380: retain atomic credential admission and constant-time unknown-friend work, without a second limiter failure count.
- #366/#367/#368/#370/#371/#372/#374/#387/#392: retain AAC repackaging, source-relative offsets, compatible DV-base selection, storage/cancellation checks, bounded stderr and slow-start activity together. Fit scaling precedes tone mapping; PGS composition uses the same scaled video.
- #365/#375/#389/#403/#408: keep catalog/scanner admission and transactional library saves while retaining typed validation errors and preparing root relocation outside the writer transaction.
- #365/#377/#398/#407/#412: keep the richer bounded bulk queue, atomic synchronous edits, import admission/session retention and root worker ownership. Remove the older duplicate import catalog-admission wrapper. Process shutdown joins requests before the asynchronous API group and closes dependencies afterward.
- #356/#373/#411/#412: retain safe webhook retries, attempt/duration diagnostics and root cancellation/joining together.
- #384/#389/#406/#415: preserve typed smart-filter validation and the shared filter-to-query conversion while selecting narrow playlist summaries.
- #415/#416: retain both independent allocation regression tests and one shared fixture.
- #383/#417: retain one header/CSP implementation and current browser expectations.
- Generated OpenAPI conflicts are resolved by regenerating from combined handlers. The original main v1 contract has no missing lines; all 53 changed contract lines are additive. Client guidance now describes #407 atomic bulk edits rather than the older pre-#407 behavior.

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
- Whole-server `go test -race -p 1 ./... -timeout 30m` passed: **36 packages passed, 0 failed, 7 packages have no tests; exit 0, no cached package results**. API 711.465s, scanner 377.258s, store 441.558s. No races detected.
- Windows amd64 cross-build passed; no Windows runtime/device acceptance claimed.
- Cached earlier-runtime upgrade and rollback checks passed for all three UIDs, preserving profiles/settings/libraries and uploaded fixture files through upgrade and a full stopped-directory rollback snapshot. This is isolated fixture acceptance, not owner deployment acceptance.
- Final fresh full pinned-browser run passed **167/167, 0 failed, 0 skipped, 0 flaky, exit 0, 16.9 minutes, retries=0**. The corrected proxy and Cast tests both passed in this complete run. The earlier 165/167 run remains a failed attempt, retained separately.

## Scope and limits

Only temporary databases, generated media, local provider fixtures and disposable container volumes. No owner data, keys, Unraid or client repositories. Every shell uses the prepared Go 1.27 and Jellyfin FFmpeg 8.1.3 activation; browser is the prepared Chromium build 1248/Chrome156, no substitute or new install.

Container validation uses the already-tested pinned/nonroot runtime image with the freshly compiled combined embedded binary copied into it, without network access. It is not a clean multi-stage Dockerfile build, multi-architecture or physical GPU/NAS acceptance. Native HDR/DV output, physical receivers, sustained usage and production deployment remain separate.

PostgreSQL compatibility is not established here. Its PRs #337/#339 require their own integration with the additional #350 SQLite migrations/query/lifecycle changes and tested baseline/upgrade updates. Do not imply that merging the PostgreSQL PRs after this branch is automatically compatible.

## Logging

No new product logging is introduced for the rehearsal. Existing per-operation diagnostics remain; test assertions distinguish the request failure from failure of a separate required recovery action. No keys, tokens, raw URLs, request bodies or SQL arguments are added to logs.

Review this resolved composition before merging anything. The owner has not authorized a main merge or deployment.

## How to review and integrate later

1. Review the original small PRs for intent and regression coverage, then inspect the linked rehearsal merge commits for interactions and the separate fixture/documentation follow-ups.
2. This branch already contains every pinned original head. After review, choose the aggregate integration PR or replay the individual merges with the recorded resolutions; do not assume they are two independent sets of fixes.
3. If an original PR receives another commit, its new head is not automatically included here. Explicitly incorporate it and rerun affected checks before claiming acceptance of that updated composition.
4. Keep the rehearsal PR in draft until review is complete. No main merge, development-image publication or deployment is authorized by this rehearsal.

Raw isolated evidence: `/workspace/arc-cloud-evidence/integration-350/` (pinned inputs, merge-event logs, original failures/artifacts, final JSON/status, Go/race/build/container checks). The results above are portable GitHub evidence; raw workspace artifacts are supplemental.

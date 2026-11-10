# #350 integration rehearsal and review handoff

This branch combines the 67 pinned review PRs #351–#417 on main `d94283c`. It is a rehearsal for review, not permission to merge or publish a development image. Main and original PR branches remain unchanged. PostgreSQL PRs #337/#339 are separate and excluded.

## Reproduce the composition

Each original head is retained as an ancestor through a local merge commit, in the order below. There were 29 conflicting merges across 20 files. Their resolutions are part of the corresponding merge commits; later test/documentation adjustments are separate. Replaying only the original PR heads on main does not reproduce the resolved tree automatically.

| PR | Pinned head | Rehearsal merge | Merge outcome |
|---|---|---|---|
| [#351](https://github.com/arc-media-server/arc-media-server/pull/351) | `c3514fcaf785` | [83a72fbcd57d](https://github.com/arc-media-server/arc-media-server/commit/83a72fbcd57d64b45f75b650d8910aa47f7a5fee) | Clean |
| [#352](https://github.com/arc-media-server/arc-media-server/pull/352) | `19604c5ad246` | [5f2e218c3999](https://github.com/arc-media-server/arc-media-server/commit/5f2e218c3999e706b154888f5e0452594a14edd8) | Clean |
| [#353](https://github.com/arc-media-server/arc-media-server/pull/353) | `a00dd498df98` | [7d0d452079bc](https://github.com/arc-media-server/arc-media-server/commit/7d0d452079bc0a6bacdcba3f275d2c561290fcca) | Clean |
| [#354](https://github.com/arc-media-server/arc-media-server/pull/354) | `2ea222d40784` | [b229fe506d79](https://github.com/arc-media-server/arc-media-server/commit/b229fe506d798af114262525bfd7c8d5ca696cbb) | Clean |
| [#355](https://github.com/arc-media-server/arc-media-server/pull/355) | `15643ef6c2dd` | [bc955d9af56e](https://github.com/arc-media-server/arc-media-server/commit/bc955d9af56eccc35a1f02b4150ec2a91e667643) | Clean |
| [#356](https://github.com/arc-media-server/arc-media-server/pull/356) | `305930cf72b5` | [aceecd6ec8ec](https://github.com/arc-media-server/arc-media-server/commit/aceecd6ec8ec2f9da46e86d89350c381c3e50683) | Clean |
| [#357](https://github.com/arc-media-server/arc-media-server/pull/357) | `cc0d26009407` | [e08cf960adb0](https://github.com/arc-media-server/arc-media-server/commit/e08cf960adb087f5c532e20b79bdcf803b6f648c) | Clean |
| [#358](https://github.com/arc-media-server/arc-media-server/pull/358) | `494dae850940` | [d920fcf93dd6](https://github.com/arc-media-server/arc-media-server/commit/d920fcf93dd64c1c3aef2e496003e485fd809ffb) | Clean |
| [#359](https://github.com/arc-media-server/arc-media-server/pull/359) | `75a54530a7dc` | [4ee650c293fd](https://github.com/arc-media-server/arc-media-server/commit/4ee650c293fd3a11b061a21744122f073c664118) | Clean |
| [#360](https://github.com/arc-media-server/arc-media-server/pull/360) | `871922366413` | [79970415d57a](https://github.com/arc-media-server/arc-media-server/commit/79970415d57a66c4a053d6645cf235941b22f2a3) | Clean |
| [#361](https://github.com/arc-media-server/arc-media-server/pull/361) | `60aad54ceb7c` | [532332aeb213](https://github.com/arc-media-server/arc-media-server/commit/532332aeb2135c73e654e322992b448b91a16adf) | Resolved: `internal/scanner/match_fixture_test.go` |
| [#362](https://github.com/arc-media-server/arc-media-server/pull/362) | `82ce87337121` | [9be59d6098d1](https://github.com/arc-media-server/arc-media-server/commit/9be59d6098d1d874d7b386a8ec8fb9118471e282) | Clean |
| [#363](https://github.com/arc-media-server/arc-media-server/pull/363) | `3c8cc271a525` | [4f5236a1eb06](https://github.com/arc-media-server/arc-media-server/commit/4f5236a1eb06b45d604b973378be0c0b24eb8ece) | Clean |
| [#364](https://github.com/arc-media-server/arc-media-server/pull/364) | `2a0575eb7eb3` | [942f2b0c72dc](https://github.com/arc-media-server/arc-media-server/commit/942f2b0c72dc37eaf21d146acd2a85f8cd92385a) | Clean |
| [#365](https://github.com/arc-media-server/arc-media-server/pull/365) | `d12b43f0f943` | [7e64b987261e](https://github.com/arc-media-server/arc-media-server/commit/7e64b987261e7ca0220ae80bdbb73f074f6b13b0) | Clean |
| [#366](https://github.com/arc-media-server/arc-media-server/pull/366) | `d09ad8023c16` | [c0516045b9c9](https://github.com/arc-media-server/arc-media-server/commit/c0516045b9c9ebea753dab507b629d7f19e2ee70) | Clean |
| [#367](https://github.com/arc-media-server/arc-media-server/pull/367) | `1cac48c33f05` | [14cb58a3853d](https://github.com/arc-media-server/arc-media-server/commit/14cb58a3853d4ab6537828c8e4ea9432a25565ba) | Clean |
| [#368](https://github.com/arc-media-server/arc-media-server/pull/368) | `1000e0b23ebf` | [725992f221f1](https://github.com/arc-media-server/arc-media-server/commit/725992f221f11f85d3a36345f183e86969af62a8) | Clean |
| [#369](https://github.com/arc-media-server/arc-media-server/pull/369) | `8c6ca9c90ac8` | [4869368c80a9](https://github.com/arc-media-server/arc-media-server/commit/4869368c80a939ea5b9c2a5b2a6597b62a14a413) | Clean |
| [#370](https://github.com/arc-media-server/arc-media-server/pull/370) | `e8186e43ca96` | [3892ba03bd87](https://github.com/arc-media-server/arc-media-server/commit/3892ba03bd87f643a2d2ca54032ee2e8cfc1b3ba) | Resolved: `internal/transcode/transcode_test.go` |
| [#371](https://github.com/arc-media-server/arc-media-server/pull/371) | `038123dd2687` | [dfac6324f1de](https://github.com/arc-media-server/arc-media-server/commit/dfac6324f1defd4e6bf153e4dc9f72c9b9d83e71) | Clean |
| [#372](https://github.com/arc-media-server/arc-media-server/pull/372) | `0de7b6cb8c48` | [c3c10d1ce559](https://github.com/arc-media-server/arc-media-server/commit/c3c10d1ce5595ed5be6dfce5708b29cfa6ac8815) | Resolved: `docs/CLIENT-INTEGRATION.md`, `internal/api/playback.go`, `internal/transcode/transcode.go`, `web/e2e/journeys.spec.ts` |
| [#373](https://github.com/arc-media-server/arc-media-server/pull/373) | `2baba2e44d47` | [86bfb007ffe1](https://github.com/arc-media-server/arc-media-server/commit/86bfb007ffe1a36b1c1991980108dbdef0b9f32c) | Resolved: `internal/notify/notify.go` |
| [#374](https://github.com/arc-media-server/arc-media-server/pull/374) | `d85299798cce` | [e4e0698e708b](https://github.com/arc-media-server/arc-media-server/commit/e4e0698e708b0a86b661fc8f5d1d8a0bef4a3fd1) | Clean |
| [#375](https://github.com/arc-media-server/arc-media-server/pull/375) | `e87f635003fc` | [389d18e944af](https://github.com/arc-media-server/arc-media-server/commit/389d18e944af29c340cacb2505b8d188819f9bfd) | Clean |
| [#376](https://github.com/arc-media-server/arc-media-server/pull/376) | `8268e6d4db70` | [54b8163f531b](https://github.com/arc-media-server/arc-media-server/commit/54b8163f531b21a6248d98f4e0e80eae151f6aa6) | Clean |
| [#377](https://github.com/arc-media-server/arc-media-server/pull/377) | `5fe001c1fb3d` | [58e3013dc6f8](https://github.com/arc-media-server/arc-media-server/commit/58e3013dc6f8445bb5347f571341d0c3cd30dd6c) | Resolved: `docs/CLIENT-INTEGRATION.md`, `internal/api/bulk.go` |
| [#378](https://github.com/arc-media-server/arc-media-server/pull/378) | `ebe132572643` | [d102dd14b384](https://github.com/arc-media-server/arc-media-server/commit/d102dd14b384f719b1a0f0f8e4284c59a6ceafae) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#379](https://github.com/arc-media-server/arc-media-server/pull/379) | `a5f3108473eb` | [24222d43d32a](https://github.com/arc-media-server/arc-media-server/commit/24222d43d32ab95384a099e23a82ed80def04ceb) | Clean |
| [#380](https://github.com/arc-media-server/arc-media-server/pull/380) | `20204af1aa40` | [5dfd473d242f](https://github.com/arc-media-server/arc-media-server/commit/5dfd473d242f6460d4c6f589698963eafe2c6e4e) | Resolved: `internal/api/friends.go` |
| [#381](https://github.com/arc-media-server/arc-media-server/pull/381) | `33a084bce9cc` | [758af2e15eb0](https://github.com/arc-media-server/arc-media-server/commit/758af2e15eb08b74776e0c276870bbe4d11460f8) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#382](https://github.com/arc-media-server/arc-media-server/pull/382) | `ebdc35337490` | [4abb333269ec](https://github.com/arc-media-server/arc-media-server/commit/4abb333269ec72279c4d3f8369fbc40d0b4263c5) | Clean |
| [#383](https://github.com/arc-media-server/arc-media-server/pull/383) | `6c92bbfd02ce` | [ce6723199896](https://github.com/arc-media-server/arc-media-server/commit/ce67231998964010e5cbf47e0d32912c7777297f) | Clean |
| [#384](https://github.com/arc-media-server/arc-media-server/pull/384) | `271aacfe86c5` | [44a65c46067d](https://github.com/arc-media-server/arc-media-server/commit/44a65c46067dc46682c0f2fdaa17690ebbd9228a) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#385](https://github.com/arc-media-server/arc-media-server/pull/385) | `c83954692396` | [844a4f57d2f4](https://github.com/arc-media-server/arc-media-server/commit/844a4f57d2f4d6f786018e48b415cda45b1b4bca) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#386](https://github.com/arc-media-server/arc-media-server/pull/386) | `8d2fd43da491` | [548f2e8d8b36](https://github.com/arc-media-server/arc-media-server/commit/548f2e8d8b369134a8ad03cf46b43beed1f03623) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#387](https://github.com/arc-media-server/arc-media-server/pull/387) | `5438f0970575` | [863e52230edc](https://github.com/arc-media-server/arc-media-server/commit/863e52230edc192b29e410b3948ac1051840a582) | Resolved: `internal/api/playback.go` |
| [#388](https://github.com/arc-media-server/arc-media-server/pull/388) | `3e50a37f0972` | [ba1f47e4024e](https://github.com/arc-media-server/arc-media-server/commit/ba1f47e4024e7a6904fad7521f19dd819bca655a) | Resolved: `docs/CLIENT-INTEGRATION.md` |
| [#389](https://github.com/arc-media-server/arc-media-server/pull/389) | `db71931286b5` | [cd99274a802c](https://github.com/arc-media-server/arc-media-server/commit/cd99274a802c8c6d187a0830d5833264fe598429) | Resolved: `docs/CLIENT-INTEGRATION.md`, `internal/api/libraries.go` |
| [#390](https://github.com/arc-media-server/arc-media-server/pull/390) | `f6021968cb30` | [7886af618778](https://github.com/arc-media-server/arc-media-server/commit/7886af61877801266d610e24c2e56f798221a102) | Clean |
| [#391](https://github.com/arc-media-server/arc-media-server/pull/391) | `cc6623470975` | [6651e249fb8d](https://github.com/arc-media-server/arc-media-server/commit/6651e249fb8d9a2bd35a53f74f1245b386cb48c5) | Resolved: `internal/scanner/scanner.go` |
| [#392](https://github.com/arc-media-server/arc-media-server/pull/392) | `c050106471ff` | [34a17c907021](https://github.com/arc-media-server/arc-media-server/commit/34a17c907021206d8c9a345c352ab783c5eb4b76) | Resolved: `internal/transcode/transcode.go`, `internal/transcode/transcode_test.go` |
| [#393](https://github.com/arc-media-server/arc-media-server/pull/393) | `05dc0a210fdc` | [8d34f896edd4](https://github.com/arc-media-server/arc-media-server/commit/8d34f896edd43d32d7fcfb80692ddcad453f48f0) | Clean |
| [#394](https://github.com/arc-media-server/arc-media-server/pull/394) | `61ea09094169` | [89059bd95846](https://github.com/arc-media-server/arc-media-server/commit/89059bd9584633c0ac9bf2a0e25803e58833449d) | Clean |
| [#395](https://github.com/arc-media-server/arc-media-server/pull/395) | `4b06617a9c19` | [bf73d33f6052](https://github.com/arc-media-server/arc-media-server/commit/bf73d33f60523b07fa1bdc0c573baaee381d3c01) | Resolved: `internal/api/api.go` |
| [#396](https://github.com/arc-media-server/arc-media-server/pull/396) | `72ce6d6a2479` | [9ca86533cdba](https://github.com/arc-media-server/arc-media-server/commit/9ca86533cdbac200e7af15db38550a0f53be245f) | Resolved: `docs/openapi.json` |
| [#397](https://github.com/arc-media-server/arc-media-server/pull/397) | `bfadbdaf0d5c` | [c8c7c33b266f](https://github.com/arc-media-server/arc-media-server/commit/c8c7c33b266fce2b0d9ecef8526f2caf19e92fbd) | Resolved: `internal/transcode/transcode.go` |
| [#398](https://github.com/arc-media-server/arc-media-server/pull/398) | `eea16f32aed4` | [59f8c78f792f](https://github.com/arc-media-server/arc-media-server/commit/59f8c78f792ffb54e4e800886a28a5c64cffc785) | Resolved: `internal/api/api.go`, `internal/api/imports.go` |
| [#399](https://github.com/arc-media-server/arc-media-server/pull/399) | `3507e0018dd2` | [814df5eda1fe](https://github.com/arc-media-server/arc-media-server/commit/814df5eda1fe5b0a87b621024fc1d6d89bc85ed6) | Clean |
| [#400](https://github.com/arc-media-server/arc-media-server/pull/400) | `83e4c78894e6` | [273ca58bec1c](https://github.com/arc-media-server/arc-media-server/commit/273ca58bec1c53f2ead300ecbb224f50b98060e2) | Clean |
| [#401](https://github.com/arc-media-server/arc-media-server/pull/401) | `8db4cc3fe27d` | [cc184d3efc7c](https://github.com/arc-media-server/arc-media-server/commit/cc184d3efc7c0f7b11fc8971ab1a3ada9eec8e1b) | Clean |
| [#402](https://github.com/arc-media-server/arc-media-server/pull/402) | `949502ab9390` | [73c5590e4c8d](https://github.com/arc-media-server/arc-media-server/commit/73c5590e4c8d9a73edfcbb90ed28fe14d00e3474) | Resolved: `internal/api/collections.go` |
| [#403](https://github.com/arc-media-server/arc-media-server/pull/403) | `7e4ed67ad20f` | [16476a4f9e3e](https://github.com/arc-media-server/arc-media-server/commit/16476a4f9e3e9c32ad51135693e36bb3a3caad46) | Clean |
| [#404](https://github.com/arc-media-server/arc-media-server/pull/404) | `d035885cd7b1` | [7b95bcee2607](https://github.com/arc-media-server/arc-media-server/commit/7b95bcee260725cfd0e08871e7ab4e4b752f470f) | Clean |
| [#405](https://github.com/arc-media-server/arc-media-server/pull/405) | `f41053a79165` | [25d7b0dafac7](https://github.com/arc-media-server/arc-media-server/commit/25d7b0dafac7e7f867f618b26627fd6856609e7f) | Clean |
| [#406](https://github.com/arc-media-server/arc-media-server/pull/406) | `e15d60f0225e` | [6904c2582d06](https://github.com/arc-media-server/arc-media-server/commit/6904c2582d069a06b8a0a52c21f9558752140548) | Resolved: `internal/api/playlists.go` |
| [#407](https://github.com/arc-media-server/arc-media-server/pull/407) | `26ad7b9005a8` | [1ed5585d54d8](https://github.com/arc-media-server/arc-media-server/commit/1ed5585d54d8ab0a1e7dbb82289469422e00cd9d) | Clean |
| [#408](https://github.com/arc-media-server/arc-media-server/pull/408) | `38e694a75b62` | [f0b9eae7ea0d](https://github.com/arc-media-server/arc-media-server/commit/f0b9eae7ea0d586a17cc48130540c490cbf1e109) | Resolved: `internal/store/libraries.go`, `internal/store/library_settings.go` |
| [#409](https://github.com/arc-media-server/arc-media-server/pull/409) | `0f42279ad6da` | [de1fa998abb5](https://github.com/arc-media-server/arc-media-server/commit/de1fa998abb56b906fd5a6eb4cb65f823d1caf83) | Clean |
| [#410](https://github.com/arc-media-server/arc-media-server/pull/410) | `9eda25bb4fa4` | [9e3e6a7dd4d6](https://github.com/arc-media-server/arc-media-server/commit/9e3e6a7dd4d6b8a624b5f8ebb93318f06b66ac7b) | Resolved: `cmd/ams/main.go` |
| [#411](https://github.com/arc-media-server/arc-media-server/pull/411) | `e2990b0d7ea1` | [45ac446cd688](https://github.com/arc-media-server/arc-media-server/commit/45ac446cd68849a0843c2be5e61d3f22b707d013) | Resolved: `internal/notify/notify.go` |
| [#412](https://github.com/arc-media-server/arc-media-server/pull/412) | `bf3cfaeb628d` | [550986a958f0](https://github.com/arc-media-server/arc-media-server/commit/550986a958f0f5c1b0d4f67cab2f9d54c3fb3f5d) | Resolved: `cmd/ams/main.go`, `docs/openapi.json`, `internal/api/api.go`, `internal/api/bulk.go`, `internal/api/imports.go`, `internal/notify/notify.go` |
| [#413](https://github.com/arc-media-server/arc-media-server/pull/413) | `1358ee07b247` | [bfa273443a84](https://github.com/arc-media-server/arc-media-server/commit/bfa273443a844863cec4168d5f06dca77f0918f3) | Resolved: `internal/api/api.go`, `internal/api/playback.go` |
| [#414](https://github.com/arc-media-server/arc-media-server/pull/414) | `c62bb02ef6cf` | [61443725d6f3](https://github.com/arc-media-server/arc-media-server/commit/61443725d6f3d92a5e76b1a5fa8ded70f892c1f7) | Clean |
| [#415](https://github.com/arc-media-server/arc-media-server/pull/415) | `db3c777e4fa2` | [e586a180e157](https://github.com/arc-media-server/arc-media-server/commit/e586a180e15728cfcb6575de984b62c5fbc6a092) | Resolved: `internal/api/playlists.go` |
| [#416](https://github.com/arc-media-server/arc-media-server/pull/416) | `53bdf07f281e` | [4dc71c58ac23](https://github.com/arc-media-server/arc-media-server/commit/4dc71c58ac2304a5f0e74e859696b006087b57b3) | Resolved: `internal/api/review_hot_paths_test.go` |
| [#417](https://github.com/arc-media-server/arc-media-server/pull/417) | `cac01600306b` | [a8b55096ad73](https://github.com/arc-media-server/arc-media-server/commit/a8b55096ad73dbf132f7e778fb9fb8b713244abd) | Clean |

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

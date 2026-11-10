# Share subtitle extraction and retain recent failures

Internal text tracks now share one FFmpeg input pass, in deterministic groups of
up to 32 tracks. Other tracks in the group are cached before another selection
needs them. External subtitle conversion (including Windows-1252) and ASS scripts/
font handling retain their existing paths. Legacy cue-cache adoption is retained.

At most eight extraction jobs are admitted, with two active FFmpeg processes and
a ten-minute root-owned deadline. Each internal output is capped and oversized/
incomplete output is rejected. A failed revision has a one-minute retry delay;
the in-memory failure hints are capped at 256. Changing file size/mtime permits
a new attempt. A changed FFmpeg setting may wait up to that minute before retry.

HTTP cancellation stops that caller's wait, leaving useful extraction work alive.
Root cancellation and Cache.Close stop admission, kill/reap processes and join
the jobs. Main supplies the process context and closes the cache before the store.
One safe warning per admitted failed group records file ID, track count, reason
and duration; repeated requests and normal shutdown do not repeat it. JSON and VTT
requests use the same diagnostic owner, without raw stderr/paths/URLs or keys.

Red regressions reproduced two passes for two tracks and immediate repeated
attempts after failure. Generated-media tests use pinned Jellyfin FFmpeg
8.1.3-Jellyfin (Docker package 8.1.3-2). Tests check both tracks, retry expiry,
changed revisions, log privacy, continued work after waiter cancellation, and
process cancellation/join on Close. External CP1252, legacy cues and ASS tests pass.

Limits: a failed batch fails its group rather than silently publishing partial
success; more than 32 tracks require multiple groups. No claim about large-NAS
speedup or physical-device subtitle rendering is made. Other process workers
still need the separate #350 shutdown changes. No API/contract, schema, dependency
or frontend changes. Preserve #351 attachment safety, #363 downloaded-cache
ownership, and the other playback.go fixes during integration.

Validation: go run ./tools/apidoc, go vet ./..., go test ./... and serial touched
race checks passed: subtitles 1.368s, API 247.430s; cmd/ams has no tests. Additional
admission/failure-hint bound tests passed in the full Go run and subtitle race
(1.398s). An admission cache recheck prevents a just-published cache entry causing
a redundant pass; its final checks supersede the retained earlier pass logs.

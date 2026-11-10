# #350: ADTS AAC and failed converted playlists

Reproduced both copied-video and encoded-video AAC failures using generated MPEG-TS with the
Docker image's pinned Jellyfin FFmpeg 8.1.3-2 package (binary 8.1.3-Jellyfin). Copied AAC now uses
`aac_adtstoasc`; copied AC-3/E-AC-3 remain unchanged. The playback adapter supplies the copied
codec, so this also covers the remux-to-convert fallback.

A known failed conversion no longer serves a stale partial playlist, including failures after its
first segment. Existing HTTP error handling reports the failure rather than returning an endless
EVENT playlist. Readiness refreshes the idle timestamp while waiting for the first segment.
FFmpeg stderr retains at most 16 KiB (first and last halves); the existing single failure warning
contains bounded first-cause and final-outcome lines. Diagnostic URLs and the full input path are
redacted. There are no new segment/progress log lines or per-poll warnings.

Regressions failed before the fix on this FFmpeg build, then passed. Additional tests cover AAC
versus Dolby filtering, partial-playlist failure, diagnostic capacity/redaction and readiness
liveness. Generated clips and temporary caches only. No hardware encoder, physical television,
AVPlayer or Media3 acceptance claimed. A conversion can still fail after readiness was returned;
subsequent playlist requests now surface that failure.

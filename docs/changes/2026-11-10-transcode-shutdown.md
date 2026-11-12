# Join stopped conversion sessions during shutdown

A stopped session leaves the active map before FFmpeg and output cleanup finish.
The manager's shutdown loop could therefore return while an already-stopped
process or cleanup was still running. It could also admit a new session afterward.

Close now stops admission under the existing lock, cancels active conversions,
and joins every process waiter, throttler/output watcher and cleanup job, including
sessions removed by Stop. Run calls Close on root cancellation, and main defers
Close for startup-error paths too. The process WaitDelay bounds inherited stderr
pipe waiting. Cleanup failures produce one safe warning with the session ID only.

Deterministic red regressions reproduced early return with a previously stopped
session and post-stop admission. Green tests gate delayed process exit, check
cleanup completion and use a generated clip with Jellyfin FFmpeg 8.1.3 to verify
actual process/watcher reaping and idempotent Close. No media files are changed.

Validation: go vet ./..., go test ./... and serial race tests for
internal/transcode and cmd/ams passed (7.781s transcode race; cmd has no tests).
The prepared binary is ffmpeg 8.1.3-Jellyfin (Docker package 8.1.3-2).
No physical GPU, Windows or container-stop acceptance is claimed.

Part of #350 API Low 9. This covers conversion sessions; joining other process
workers and hardware-detection jobs remains separate. Preserve the other playback
fixes when integrating transcode.go. No API/schema, frontend or dependency change.

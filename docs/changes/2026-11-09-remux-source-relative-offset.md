# #350: remux positions use the source-relative timeline

Reproduced with the Docker image's pinned Jellyfin FFmpeg 8.1.3-2 package (binary
8.1.3-Jellyfin): a generated MPEG-TS source starting about 1001 seconds into its container
timeline returned offset_ms=1010148 for a nine-second seek. Player progress/resume should
refer to the title's own timeline, not its container timestamp.

For seeked copied-video sessions, Ready now reads both output and source start_time and
subtracts the source origin. Clamp the result to zero and the known source duration. If either
probe cannot establish a finite timeline, retain the requested offset. Both probes share the
existing 15-second timeout; no schema, API field or media changes. Encoded and zero-start
sessions retain their existing behavior.

The generated shifted-TS regression failed before the fix, then passed; existing H.264 and
HEVC MKV keyframe controls also passed. Tests allow keyframe/audio preroll around the requested
position rather than assuming frame-exact seeking. No physical-player, hardware or owner-server
acceptance claimed. Existing session outcome logging remains sufficient; the extra bounded probe
runs once per seeked remux, with no per-segment logs or raw command/error URL logging added.

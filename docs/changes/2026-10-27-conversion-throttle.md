# Conversions wait for the viewer (#193, 27 October)

Live conversions (remux and transcode sessions) used to run FFmpeg flat out to the end of the
file, however far the viewer had got. Now, as Plex's throttle buffer, a session converts at full
speed until it is 2 minutes ahead of the segment the player (or cast device) last fetched, then
FFmpeg is paused (no CPU, GPU or disk use) until the player is within 1 minute, or asks for a
segment not made yet (resumed at once). A viewer who pauses or gives up no longer leaves a whole
film being converted. Seeking past what's converted still starts a new session, as before.

- Linux/macOS (and Docker/Unraid): SIGSTOP/SIGCONT. Windows: NtSuspendProcess/NtResumeProcess
  (compiled, not run here). If pausing fails, the session runs at full speed as before.
- Dashboard → Now playing shows "Converting (throttled)" while a session waits (playing API:
  `throttled`). No setting: Plex's Throttle Buffer is a fixed sensible default here.
- Optimized versions and background work are unchanged (they are whole-file jobs by design).

Measured locally (4 vCPU, libx264 veryfast, generated 10-minute 720p clip, simulated real-time
viewer with 30 s buffered), first 60 seconds: without throttling FFmpeg used 152.9 s of CPU,
wrote 184.9 MB and was 456 s ahead; with it, 53.2 s of CPU, 62.4 MB, 114 s ahead and paused.
After the first burst a session uses about real-time effort. Not measured on the owner's server.

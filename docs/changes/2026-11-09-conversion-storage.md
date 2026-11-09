# #350: converted-stream storage safeguards

Check free space before conversion starts and every five seconds for active sessions. Below a
512 MiB reserve, refuse with retryable 503 or cancel the running conversion with one safe
outcome warning. Check once per configured root per pass, outside the manager lock. Unsupported
free-space checks warn once and retain normal filesystem write-error handling. This is a best-effort
guard, not a reservation against other processes consuming disk between checks.

Add optional playback request/plan `rolling_hls`. Existing clients/native-HLS browsers keep their
full EVENT playlist behavior; hls.js web playback opts in. Successful segment requests retain two
minutes behind the furthest requested position and delete older segments, preserving init/current/
forward segments. Nonexistent/future requests cannot delete history; retention only advances.
Each request unlinks at most 64 files; open-file failures are retried later (including Windows).
Serve a sliding playlist with the matching media sequence and without the EVENT promise. Keep the
producer's small playlist for pacing/completed-position arithmetic. The web seek handler starts a
fresh conversion when a target is before the moving window, so old seeks still work.

Legacy full caches remain until session cleanup and are guarded by free-space admission/monitoring;
native clients may opt in only after implementing restart-on-outside-window seeking. Native Safari
remains on its existing full-playlist path. No setting or client-breaking default introduced.

Tests cover opt-in versus legacy, nonexistent future requests, retained/forward/init files,
low-capacity admission and one-time running cancellation. Real generated H.264 conversion with
Docker-pinned Jellyfin FFmpeg 8.1.3-2 (8.1.3-Jellyfin binary) advances past retention, deletes old
segments and leaves a served window FFprobe can read. Browser checks exercise actual hls.js
playback, forward seek/resume and restart for seeking behind the window. Temporary stores/clips/
local provider fixtures only. No owner filesystem, real-device/native-app or physical low-disk
acceptance claimed. Decoder buffers/prefetch mean retention is relative to segment requests,
not proof those frames were watched; watch progress remains a separate API.

No per-segment cleanup logs. Low-space stop logs once per session with free/reserve byte counts;
unavailable checks warn once per manager with no path/raw errors. No keys/tokens/bodies/raw URLs
or SQL arguments added. API additions are regenerated with tools/apidoc.

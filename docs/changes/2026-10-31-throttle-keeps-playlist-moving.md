# Throttled streams keep their playlist moving

The owner (31 October, Chromecast with Google TV): playback would spin for a long time, and the
Dashboard showed the stream as "throttled".

## Why

Throttling paused FFmpeg 2 minutes ahead of the player and resumed it only when the player got
within 1 minute. For that minute, the stream's HLS playlist didn't change at all. Arc serves
conversions as growing ("event") playlists, which players handle like live streams. Android's
Media3 treats a live playlist that hasn't changed for 3.5 target durations (about 21–30 s here)
as stuck and fails playback (`DEFAULT_PLAYLIST_STUCK_TARGET_DURATION_COEFFICIENT = 3.5`). The TV
app then asked Arc to convert more and started again, hence the long spinner. hls.js also
refreshes such playlists, and a still one only helps it stall.

## What changed

- **Resume one segment short.** FFmpeg still pauses 2 minutes ahead, but now resumes as soon as
  it's one segment (6 s) short of that, instead of a minute short.
- **The effect:** while someone watches, the playlist grows about a segment at a time. The
  2-minute ceiling, and pausing a paused viewer's conversion, are unchanged. Pausing and resuming
  FFmpeg (SIGSTOP/SIGCONT) is cheap, so the shorter cycle costs nothing noticeable.

## Checks

- The real-FFmpeg throttle test now plays one segment on after the pause and expects the
  conversion to resume and pause again about a segment later. With the old one-minute gap it stays
  paused and the test times out.
- A unit test keeps the pause/resume gap within one segment.
- Not yet confirmed on the owner's Chromecast. If a stream still spins, the TV app's log line
  "… playback failed on this device (ERROR_…)" names the error.

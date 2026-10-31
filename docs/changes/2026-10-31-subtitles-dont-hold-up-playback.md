# Subtitles inside a large file no longer hold up playback

A Dolby Vision film (23 GB, with forced English subtitles inside the file) spun for about four
minutes before playing on the Android TV app, every time. The TV's log showed why:
`Disabling track due to error: id=1:arc-s3 … text/vtt … SocketTimeoutException`, and playback
began one second later.

## Why

- The app loads its subtitles as a separate WebVTT file, and the player waits for that file before
  it starts the video.
- Reading a track out of a video the first time means reading the whole file, which takes minutes
  for a large one.
- `/vtt` answered only once the reading was done, so the player waited until its request timed
  out. Only then did it drop the subtitles and start.
- The reading was tied to the request, so the timeout also cancelled it. Nothing was cached, and
  the next play waited all over again.

## What changed

- **`/vtt` answers at once.** If the track is already read (cached, or a file beside the video), the
  whole WebVTT file comes back as before. Otherwise, after half a second, the answer starts with
  the WebVTT header, sends a blank line every 10 seconds (allowed between cues) to keep the player
  and any proxy from giving up, then sends the cues. The video starts straight away, and the
  subtitles appear once read. If the reading fails, the answer simply has no cues, and the failure
  is logged once.
- **Reading a track runs on its own.** It's shared by everyone waiting, isn't cancelled when a
  player stops waiting, and is cached when done, so the next play has the subtitles at once. It's
  still bounded at 10 minutes.

## Checks

- A new test stands in for FFmpeg with a slow script and checks three things: the header arrives
  within a second; a player giving up doesn't cancel the reading; and FFmpeg runs once, with the
  next request served from the cache. It fails on the old code (no answer before the deadline).
- The existing WebVTT and cast subtitle tests pass.

## Limits

- ASS/SSA scripts for libass (`/ass`) still wait for the whole script. They're rarer as forced
  subtitles; the same approach can follow if needed.

# Honest conversion reasons, and TV browsers that undersell H.264 (7 November)

## The problem

On an LG TV's browser (webOS 25, 2025 OLED B5), every title said "Converting… 10-bit H.264
video, MKV file", even ordinary 8-bit files, and the picture stayed black.

## What changed

- **The reason only blames 10-bit H.264 when bit depth is the cause.** The server used to label
  any rejected H.264 file that way. That included a player that didn't list H.264 at all, and the
  web player's own retry, which sends an empty video list. Such a player now gets "H.264 video",
  like any other codec it lacks.
- **Retries say they're retries.** When the browser can't play what it said it could, the web
  player asks again with narrower capabilities. It now also sends `failed_method` ("direct" or
  "remux"). The reason then reads "the player couldn't play the repackaged stream" (reason code
  `player_failed`), so the info panel and the log point at the browser, not the file. The field is
  additive and optional.
- **Retries also ask for the safest audio.** The retry used to narrow only the container and
  video, so a file whose Dolby Digital (Plus) audio the browser had over-claimed failed the same
  way twice. LG's webOS browser does this, and it's the likely cause of that TV's black screen.
  Retries now ask for AAC (or MP3), so the server converts the audio too.
- **H.264 and AAC count when a browser only answers "maybe"**, or only for a plainer H.264 profile.
  Every browser that plays video plays these in MP4. Some TV browsers answer the stricter question
  "no", and Arc then converted every file on the server's CPU.
- **Detailed logging** records the capabilities a player sent whenever Arc converts. This tells a
  conversion caused by the browser's answer from one caused by the file.

## Limits

- Not checked on a real LG TV yet. It isn't known whether the LG browser then plays the direct,
  repackaged or converted stream. The new reasons and the debug line will show which step fails.
- The black screen itself (a stream the browser accepts but never shows) still ends in the
  player's existing error after its retries.

## Logging

- **Info:** the existing "playback" line, now with honest reasons.
- **Debug, conversions only:** "playback caps", with the file id and the reported containers,
  codecs, max height and bit depth. It holds no URLs or user data, and is written once per
  playback start.

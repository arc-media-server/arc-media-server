# Retrying a failed stream works again; the throttle can't hold a playlist still

Found while building the Apple TV app (arc-apple) against a scratch Arc.

## What changed

- **Retries after a failed stream work again.** When a file played as it is fails, the web player
  and the Android app ask again with empty lists: no containers ("repackage it"), then no video
  codecs ("convert the video"). The capability checks added with playback previews (30 October)
  read those empty lists as "can't play MP4 or H.264", so both retries were refused with 422 ("player
  cannot play this server's conversion output"). A failed direct play then ended in an error
  instead of a repackaged or converted stream.
  - An empty list is now read as the players mean it. A list that names other formats but not
    MP4 or H.264 (e.g. only `mkv`, `hevc`) is still refused, as the preview work intended.
- **The throttle can't hold a playlist still any more.** A fast conversion (a remux especially)
  can finish several segments between two throttle checks, so it pauses well past two minutes
  ahead. One segment of progress from the player then wasn't enough to resume it. The playlist
  stood still for several segment lengths, which players treat as stuck: Media3 fails a live
  playlist after 3.5 target durations.
  - Now, whenever the player moves on while the conversion is paused, it resumes, and it makes
    at least one more segment before it pauses again.
  - `TestConversionWaitsForThePlayer`, which had been failing on main, passes.
- **docs/API.md** describes the WebVTT endpoint's streamed answer (#324). The endpoint writes with
  `w.Write` again, so the API generator sees its success answer and `TestV1ContractKept` passes.
  It had been failing on main.
- `internal/api/playing.go` is gofmt'd.

## Checks

- `go vet ./...` and `go test ./...` pass (they had two failures on main).
- `TestFallbackCapsStillConvert` fails on the old code and passes now.
- Against a scratch Arc built from this branch:
  - the converted retry answered 200;
  - the Apple TV client's live test (arc-apple, ArcKit `LiveServerTests`) played a direct file
    and a converted stream through cast links.

  On main the same request answered 422.

## Logging

No new logging. A refused request is still an ordinary 422 with its reason codes. The throttle's
pause and resume points are visible on the Dashboard ("throttled") as before.

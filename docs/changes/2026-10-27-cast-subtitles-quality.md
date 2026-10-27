# Chromecast subtitles and original quality (#192, 27 October)

**Text subtitles on Chromecast.** Casting with a text subtitle on no longer stops with a
message. The device shows the track itself: Arc sends it as WebVTT through the cast link
(new GET /api/items/{id}/subtitles/{track}/vtt?file=&shift_ms=). Cue times are shifted for a
converted stream's start point and the viewer's ±10s timing adjustment, and the link stops
working when the sender's session is revoked, like the video. Picture subtitles are still
burned in. AirPlay keeps its explanation (Safari doesn't send page-drawn captions to Apple TV).

**Original quality first, then fall back.** Google's Default Media Receiver can't report what
a device decodes, so Arc first offers the original (HEVC/AV1, up to 4K, Dolby Digital/DD+
audio). If the device refuses the load, or stops with an error before it has played, Arc
retries with what every Chromecast plays (H.264/VP9 up to 1080p, as before) and says so. The
result is remembered per device name in this browser; a device that plays 5 s of the original
is remembered as able to. A custom Arc receiver that asks the device directly can follow once
the repo is public (it needs a registered receiver app and a public HTTPS page).

Checks: go vet/test (new WebVTT and cast-link tests), tsc app/e2e, production build, Playwright
webserver, preplay, adaptive, journeys and ux specs (69 journeys, including new subtitle-track
and fallback-and-remember journeys). These use SDK stand-ins only; real Chromecast/Google TV
behaviour is the owner's device test on #192.

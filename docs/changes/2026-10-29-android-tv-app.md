# Android TV app: first working slice

Milestone 18's native apps (#63) start with Android TV / Google TV, as the owner chose: a native
Kotlin app (Jetpack Compose for TV, Media3 ExoPlayer) in `clients/android-tv/`, in this repository
so one PR can change the API and the app together. It talks only to `/api/v1`. The server and web
app are unchanged; `clients/` is added to `.dockerignore` so the image build doesn't see it.

## What works

- **Server address.** Typed with the remote; `192.168.1.20` becomes `http://192.168.1.20:8099`,
  an address with a scheme or port keeps them (`https://arc.example.com` uses 443). Checked with
  `GET /health` (shows the version and API version), remembered. A web page that isn't Arc is
  told apart from an unreachable server.
- **Sign-in with a code** (as Plex's plex.tv/link): the TV shows the four-character code from
  `POST /auth/link`, says where to enter it (profile menu, Link a Device, or `<server>/link`),
  polls every 3 s, gets a fresh code when one runs out, and keeps the device token. The token is
  encrypted with an AES key held in the Android Keystore (non-exportable), not stored in clear.
  A token signed out from Settings > Devices sends the TV back to the code screen.
- **Profiles.** "Who's watching?" with the web's avatars and colours; a PIN profile opens a number
  pad (remote number keys work too) that checks as soon as four digits are in; a wrong PIN turns
  the dots red and clears them. The profile stays chosen on the server for the token, so the next launch goes
  straight to Home (as Plex does); the sidebar has Switch Profile and Sign Out.
- **Home** is the profile's own layout, resolved the way `web/src/lib/home.ts` does it: saved
  order, hidden rows, built-ins not yet placed inserted where they'd be by default, Up Next inside
  Continue Watching unless the profile split it out, Watchlist, Recently Added per library (a TV
  library groups new episodes by season via `/libraries/{id}/recent-shows`), and custom filter
  rows. The focused card's backdrop, title and summary fill the top of the screen (Plex/Google TV
  style). Empty rows are left out; a row that fails shows its error and a Try Again button.
- **Libraries** in a left rail (icons that open into labels, as Plex and Google TV do), in the
  profile's library order, each an A–Z poster grid that loads more as the focus nears the end.
- **Details.** Movies and episodes: backdrop, title, year, age rating, runtime, genres, summary,
  Resume from 52:00 / Play from Beginning, and Go to Show for an episode. Shows: the same header
  with Play/Resume of the next episode (the one part-way through, else the first unwatched,
  specials last), then season tabs and episode cards. Seasons and episodes come from
  `GET /series/{id}/seasons` without an ordering, so the show's display order is the server's.
  Coming back from the player refreshes resume points and ticks.
- **Playback** mirrors the web player's contract (not its code): `POST /items/{id}/playback`
  with this device's real capabilities; plays the file as it is (range requests) or the HLS
  stream; the `offset_ms` clock (the player's position plus the offset is the file position;
  seeks inside what's converted stay in the stream, others ask for a new one); progress every
  10 s, on pause and with `stopped` on close; `POST /transcode/{sid}/ping` every 30 s; `DELETE`
  of the session when leaving or restarting; one retry with fallback caps when the device fails
  to play what it claimed (direct → repackage → convert), as `fallbackCaps` does; an admin's
  "stop playback" (403 `needs: stopped`) shows their message. Controls: play/pause, back 10 s /
  forward 30 s, a scrubbable timeline (left/right in 10 s steps), media keys, and side panels for
  the audio track (restarts the decision with `audio_stream`, as the web does) and text subtitles
  (WebVTT from `/subtitles/{track}/vtt` with `shift_ms` = −offset, chosen by default as the web's
  `chooseSubtitle` does). Controls hide after 5 s of playing.
- **Errors**: every load shows a spinner and then either the content or what went wrong in plain
  words (Arc's own `error` text when it gave one) with Try Again, which takes the focus.
- **Strings**: all user-facing text is in `res/values/strings.xml`.

## Decisions

- **minSdk 26 (Android 8.0).** Older Android TV boxes and TVs still in everyday use (Mi Box S,
  2018–2019 Sony/Philips/TCL sets, Shield on older firmware) run 8 or 9; 26 also gives
  `Display.Mode` (for the 4K check) and adaptive icons. Media3 and Compose support it.
- **compileSdk 37, targetSdk 36.** The brief said 36, but today's stable Compose BOM 2026.09,
  Navigation 2.10, Coil 3.6, Core 1.19 and OkHttp 5.5 all refuse to build against less than 37.
  compileSdk only decides which APIs the code can see; runtime behaviour follows targetSdk 36.
  `tools/setup-sdk.sh` installs platform 37.
- **Versions:** AGP 9.5.0 (built-in Kotlin) with Kotlin 2.4.21, Gradle 9.8.2, Compose BOM
  2026.09.00, tv-material 1.1.0, Media3 1.11.1 (exoplayer, hls, okhttp datasource, ui),
  OkHttp 5.5.1, kotlinx-serialization 1.12.0, Coil 3.6.3; tests with Robolectric 4.17 and
  Roborazzi 1.77.0. All stable.
- **Structure:** one `app` module; `data` (ArcApi over OkHttp + kotlinx.serialization with
  `ignoreUnknownKeys`, models holding only the fields used), `playback`, and a package per screen
  with a ViewModel exposing `StateFlow` and a stateless composable (which the screenshot tests
  render). A small `AppContainer` instead of a DI library.
- **Focus and navigation:** Plex's pattern: a left rail that expands on focus, Home rows below a
  hero of the focused title, cards that grow 8 % with a white outline and violet glow, the first
  card or primary button focused when a screen opens, Back closes panels/controls before leaving.
  Episode cards play straight away (resuming); the details page offers Play from Beginning.
- **Capabilities** are measured, not listed per model: video and audio codecs from
  `MediaCodecList` decoders (h264, hevc, vp9, av1, mpeg2video, mpeg4; aac, mp3, opus, flac, vorbis,
  ac3, eac3, dts, truehd), plus Dolby/DTS formats the TV or receiver takes by HDMI passthrough
  (`AudioCapabilities`), containers Media3 opens itself (mp4, mkv, webm, mpegts) and
  `max_height` from the tallest display mode (a 4K TV often runs its UI at 1080p).
- **Converted streams play from their start at normal speed:** Arc's HLS playlist is an "event"
  playlist that grows while converting, which ExoPlayer treats as live; the app pins the live
  playback speed to 1 and gives an explicit start position so it never races to the live edge.
- **Artwork and stream URLs** Arc returns as `/api/...` are rewritten to `/api/v1/...` (the same
  handlers), so the app only ever calls the stable API. The token goes only to the Arc host;
  artwork redirected to TMDB doesn't carry it.
- **Cleartext HTTP is allowed** (network security config): Arc at home is plain HTTP on whatever
  address the person types.
- **Release builds are debug-signed** until there's a signing setup; no keys in git.
- **Screenshots aren't committed** (about 8 MB of PNGs); `./gradlew recordRoborazziDebug`
  regenerates them.

## Logging

`android.util.Log` with the tag `Arc` (`adb logcat -s Arc`). Info: connected to a server (name,
version, API version), signed in with a code, a code ran out, signed out, a stale token, and each
playback decision with its reasons ("Arc is converting: DTS audio (NVIDIA NVENC)") or a fallback
after the device failed ("direct playback failed on this device (ERROR_CODE_…); asking Arc to
convert more"). Warn: every failure with what failed (Home row id, library id, details, playback
decision status, progress saving, a stored token that can't be decrypted), as the exception's
class and HTTP status. Debug (only with `setprop log.tag.Arc DEBUG`): routes answering 4xx, Home
load timings, failed keep-alive pings. Never the token, PINs, passwords or full URLs (routes are
logged as patterns like `items/{id}/playback`). Nothing is logged per frame, per segment or per
progress report.

## Checks run

- `./gradlew assembleDebug testDebugUnitTest recordRoborazziDebug`: build OK, 33 unit tests and 12
  screenshot tests pass (the live test is skipped without a server).
- End to end against a scratch Arc (this branch's server, the `tools/testmedia` library, an admin
  and a "Sam" profile with a PIN): `LiveServerTest` passed — health (API v1); sign-in code shown,
  approved as the admin over the API, polled to a token, the poll then answering 404; profiles
  listed, wrong PIN refused (401, needs pin), right PIN accepted; libraries (7 movies, 3 shows);
  every Home row loaded (Continue Watching, Watchlist, Recently Added ×2); movie and show lists;
  Firefly's seasons in display order (S1: 15, specials: 1); The Matrix decided as direct play with
  a 206 range read of the stream; a Firefly episode with no containers accepted came back as a
  repackaged HLS session at `offset_ms` 1000, its playlist and first fMP4 segment fetched under
  `/api/v1/transcode/...` with the token; progress, ping, stop, session ended; the episode then
  marked watched, the show's next episode moved on, and Continue Watching offered it as Up Next.
- Root checks: `go vet ./...`, `go test ./...`, `cd web && npx tsc -b && npm run build`.

Not checked: real playback on a TV or the emulator (no KVM here), so ExoPlayer, decoders, HDR,
passthrough and D-pad feel are untested on hardware — #135-style real-device checks are still
needed before calling this done.

## Limits and what's next

- Not yet: next-episode autoplay and Skip Intro/Credits (markers are in the plan), picture
  subtitles (need `burn_subtitle`), quality choice and remote-bandwidth handling, versions and
  editions, trickplay thumbnails while scrubbing, search, collections and playlists rows, the
  seasonal row, Other videos' folder browsing (its library shows a flat grid), watchlist/played
  toggles, spoiler hiding, saving the chosen subtitle language back to the profile, theme music,
  TMDB/TVDB attribution screen, Android TV "Watch Next"/channels integration, and a signed release.
- Next: real-device pass (Chromecast with Google TV, Shield), next episode + skip markers, picture
  subtitles, search, then the iOS/tvOS app on the same API.

## v1 API gaps found

1. **Playback and artwork URLs come back unversioned** (`/api/items/.../stream`,
   `/api/transcode/.../index.m3u8`, `/api/items/.../images/...`). Apps must rewrite them to
   `/api/v1/` to stay on the stable API. Answering `/api/v1/...` URLs to `/api/v1` requests (or
   documenting that these paths are stable) would close it.
2. **`caps` can't say what the screen and sound system do**: no HDR10/HLG/Dolby Vision support
   (so a TV without HDR gets HDR files as they are, unless Android tone-maps), no maximum audio
   channels, no codec profiles/levels/bit depth (e.g. 10-bit HEVC or high AV1 levels), no
   maximum bitrate. Optional fields like `hdr: ["hdr10","hlg","dv"]`, `max_audio_channels`,
   `video_profiles` would let Arc decide better for TVs.
3. **No server-resolved Home**: each app has to reimplement `resolveRows` (saved order, defaults,
   seasonal dates, collection/playlist rows). A `GET /api/v1/home` returning the profile's rows
   with titles and item lists would keep apps consistent.
4. **List items have no duration**, so progress bars use `runtime_min` (missing for unidentified
   titles); `duration_ms` on movies/episodes in lists would fix that.
5. **The WebVTT subtitle endpoint is documented as for cast devices**; native players need it too
   (and with `shift_ms` for converted streams). Worth documenting as the way apps get text
   subtitles.
6. The HLS playlist stays an "event" playlist without `#EXT-X-ENDLIST` until conversion ends, so
   native players see it as live; documenting this (or sending `EXT-X-START`) would help other
   app authors.

# Internet streaming quality, per profile

The owner asked for profiles that watch away from home to be able to set their own maximum
streaming quality, like Plex's "Internet Streaming: Video quality".

What changed:
- **Settings, Your profile, Playback: "Internet streaming quality".** Maximum (the default) or one
  of the player's quality limits (20 Mbps 1080p down to 720 kbps 360p), with Plex's explanation:
  "Set the default quality for streaming video over the internet. If quality is set too high,
  videos will start slowly and pause frequently." It adds that it only applies away from home.
  It is a server-side profile preference (`remote_quality_kbps` in `PATCH /api/me/prefs`), so it
  follows the profile to every device. The server accepts only 0 (Maximum) or a value from the
  qualities list (400 otherwise); Go keeps the list in `qualityLimits` beside `qualityHeight`,
  matching `qualities` in `web/src/lib/playback.ts`.
- **Applied by the server when a stream is away from home.** `remoteLimit` (used by
  `POST /api/items/{id}/playback`) now takes the lowest of the server owner's "Limit each remote
  stream", the share of the upload speed, and the profile's choice. On the home network (including
  the server owner's home networks and trusted proxies) none of them apply, as before.
- **The player's quality menu shows the cap honestly.** It already received the cap as
  `server_limit_kbps`, but only mentioned it in the Automatic notice and still offered higher
  choices that the server quietly lowered. Now, when the playing file is above the cap, choices
  above it are left out, "Original" reads "Maximum (4 Mbps 720p)", and the menu says "Away from
  home, streams are limited to 4 Mbps 720p." A viewer can still pick lower. A device choice saved
  above the cap (e.g. at home) shows as Maximum while away.

## "Play smaller videos at original quality": not added

Plex has a checkbox for this because, without it, Plex converts every video to the chosen quality.
Arc already decides this by itself: the playback decision converts only when the file's bitrate is
above the limit (with 10% leeway) or its picture is taller than the limit's resolution; a file
already within the limit plays untouched (direct play, or a repackage if the player needs one).
There's no case where converting a file that is already smaller is better, so a checkbox would be
an option without a reason (ease over options). A Go test covers it
(`TestPlaybackHonoursRemoteQuality`). For the owner's decisions list (#80): the coordinator will
log this there.

## Differences from Plex

- **No separate "Home streaming" quality.** Plex also has a per-device home quality; Arc's player
  already has its per-device quality menu (Automatic, Original or a limit) for that, and Automatic
  adapts to the connection. Possible follow-up if profiles want a home limit too.
- **Plex's setting is per device (per app);** Arc's is per profile so it follows the person. The
  player's own quality menu remains the per-device choice within it.
- The server owner's "Limit each remote stream" still applies on top: the lower of the two wins.

## Checks

- Go: `TestRemoteQualityPrefValidation`, `TestRemoteLimitUsesProfileQuality` (remote takes the lower
  of both, home ignores both), `TestPlaybackHonoursRemoteQuality` (capped away, smaller file plays
  directly, home unaffected).
- Browser: `ux.spec.ts` "internet streaming quality is saved to the profile..." (save, reload,
  invalid value refused, playback decision away vs home through an untrusted `X-Forwarded-For`)
  and `adaptive.spec.ts` "the profile internet streaming quality caps a stream away from home..."
  (real conversion at the cap, menu shows "Maximum (2 Mbps 720p)" and no higher choices).
- Results (29 October): `go vet ./...` and `go test ./...` pass; `npx tsc -b`, the e2e type-check
  and the production build (with the i18n audit) pass; the full browser suite ran 129 passed,
  1 failed. The failure, "row navigation: visible mouse controls reach beyond the first screen",
  fails the same way on a clean `origin/main` checkout on this machine, so it isn't from this change.

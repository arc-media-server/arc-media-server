# Pre-roll clips in native clients

For an initial movie play from the beginning, fetch
`GET /api/v1/items/{id}/preroll` with the device session's Bearer header. The response is an
ordered array of `{title, url}`; it can be empty. Server Playback Settings choose the configured
clips and whether to return all or one at random. Do not fetch/play pre-rolls on resume,
Watch Together or automatic queue advancement. A list failure or three-second client deadline
must fall through to the film; late results must not change playback.

## Players without request headers

Pass each returned URL unchanged to `POST /api/v1/cast` as `{"url": "..."}`, using normal
Bearer authentication. Give the returned `url` to the native player. This is the same handoff
used for ordinary playback and extras; session credentials must never appear in player URLs.
Only configured regular pre-roll video files are accepted. No new route or response fields
are introduced; accepting pre-roll URLs is additive to the existing cast API.

A pre-roll grant authorizes one configured clip at its current list index. It cannot open
another clip, an item, an extra, conversion output or a settings endpoint. Existing cast expiry
and session revocation apply. Removing/reordering configured clips invalidates a link when its
index no longer names the same path. The grant binds the configured path, not a file-content
snapshot. Raw filesystem paths stay server-side. Byte-range reads are supported.

Play clips in response order. A failed or unsupported clip advances to the next; after the last
clip, start the requested film. The existing Skip control advances one clip, while Back stops
the entire attempt. Do not send film progress or watched updates for pre-rolls. Ignore late clip
callbacks after switching; cancellation must also stop a native open that completes after Back.

## Acceptance limits

HTTP tests cover authenticated link creation, ordinary unauthenticated route refusal, Range,
scope isolation, invalid indices, configuration reorder and session revocation. Native clients
still need their own rendering, remote/focus, callback-race and physical-device checks. The
server does not transcode or establish a clip's codec compatibility with a device.

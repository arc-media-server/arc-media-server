# Watch Together — backend and client handoff

Server contract for web and native clients using `/api/v1`. For the first room creation/join
requests, see [the client walkthrough](CLIENT-INTEGRATION.md); the full synchronization and
recovery contract is below. Native-client implementation and device acceptance are separate.
Rooms synchronize **item, timeline and playback intent**; every participant still starts their
own ordinary Arc playback. Nothing here creates a conversion, changes personal preferences or
writes watch progress/history. There is no cloud discovery service or external infrastructure.

## Capability and authentication

`GET /api/v1/capabilities` includes `features.watch_together` with supported/available/allowed.
Signed-in household profiles and existing friends are allowed by default. Item library, age and
label restrictions still apply. A participant is a **device session plus its selected profile**:
two devices using the same profile are two participants. Switching profiles does not inherit the
previous profile's membership. Never put Bearer tokens or invite codes in URLs.

Rooms are private. There is no global room list or admin inspection endpoint. `GET .../current`
returns only this exact device/profile's own room, so a reload or lost create response can recover
it. Room IDs and participant IDs are opaque; session hashes and profile IDs are never exposed in
room payloads. Participants see the names of the other people who joined this private room.

## Endpoints

All routes below are relative to `/api/v1/watch-together` and require a selected profile.
Bodies are JSON, at most 4096 bytes, with unknown fields/trailing JSON rejected.

| Method and path | Request | Result |
| --- | --- | --- |
| POST (base path itself: `/api/v1/watch-together`) | `{item_id,start_ms?}` | 201 `{room,invite_code,invite_expires_at_ms}` |
| POST `/join` | `{code}` | 200 personalized room |
| GET `/current` | — | Own active room, or 404 |
| GET `/{id}` | — | Member-only personalized room |
| POST `/{id}/commands` | `{revision,action,position_ms?,item_id?}` | Updated room; stale revision → 409 |
| POST `/{id}/heartbeat` | `{generation,position_ms,state,ready}` | Updated room; stale generation → 409 |
| POST `/{id}/leave` | Empty | 204 |
| POST `/{id}/invite` | `{revision}` | Host-only replacement invite and room |
| GET `/{id}/events` | Auth header/cookie | SSE `room` events with full personalized room state |
| GET `/time?client_sent_ms=...` | Optional nonnegative epoch-ms integer | Client echo, server receive/send epoch ms |
| GET `/people?item_id=` | Optional item ID | `{people:[{id,name,color,friend,can_watch}]}`: who this profile may invite |
| POST `/rooms` | `{item_id,start_ms?,profile_ids}` | 201 `{room,invite_code,invite_expires_at_ms}`: a room for chosen people, nobody in it yet |
| GET `/rooms` | — | `{rooms:[...]}`: rooms this profile made, was invited to or is in (Home's Watch Together row) |
| POST `/rooms/{id}/join` | Empty | 200 personalized room: join from the list, no code |
| POST `/rooms/{id}/dismiss` | Empty | 204: an invitee declines; the creator cancels it for everyone |
| POST `/{id}/people` | `{profile_ids}` | Host-only: invite more people; personalized room with `invited` |

Commands: `play`, `pause`, `seek` (position_ms), `change_item` (item_id and optional position_ms),
`end`. Any participant who may play the current item can play/pause/seek. Any member may select
another movie/episode they may play. Only the host can end the room or renew the invite; a host
who lost access to the current title may still end it. Series IDs and unavailable media cannot
be selected directly: resolve a playable episode using the normal episode/order APIs.

Each command has the last observed **revision**. On 409:

```json
{"error":"room changed","room":{"id":"...","revision":7,"generation":2,"state":"waiting"}}
```

(The real room payload includes the other documented fields.) Adopt that state; retry only if
the user's action is still appropriate. Do not endlessly replay stale seeks or changes.
A revision can advance more than once per request because readiness/state transitions also
change it. A seek or item change increments **generation**, independent of general revisions.
Heartbeats must acknowledge the generation actually loaded/seeked; late reports from an earlier
item/seek return 409 and cannot release the new ready barrier.

## Room payload and privacy

Room fields: `id`, `revision`, `generation`, `state`, `position_ms`, `at_ms`, `rate` (currently 1),
`server_time_ms`, `host_id`, `self_id`, optional `changed_by`, optional `wait_until_ms`, `available`,
optional `item_id` and `item` summary (`id`, `title`, `kind`), and `participants`.
Each participant has `id`, `name`, `available`, `ready`, `state`.

Item summaries are deliberately small; fetch normal item details once the item/generation
changes. `changed_by` identifies who issued the latest command or membership change; use it for
"Paused by …" where appropriate. Heartbeats do not move the shared timeline or change this field.

Access is checked on admission, for every participant when changing titles, on reads/heartbeats,
and before each SSE payload. A participant denied a later title remains present, with
`available:false` and **no item ID or item metadata**. The same redaction applies to 409s and SSE.
Display "This title isn't available to you" and let them stay or leave. They do not block the ready
barrier. Other participants' availability is refreshed by their own traffic or by a title change;
media authorization is always checked freshly before revealing the requesting viewer's metadata.

Codes are returned only at create/renew, not in room GET/SSE. Wrong, expired and restricted
invitations all return the same 404 shape. Unauthorized/nonmember IDs are 404 even for admins.
Room API responses use private/no-store. Successful room payloads and conflicts include server
current time; the response header `X-Arc-Server-Time-Ms` is also supplied at request handling,
including empty/error responses. Prefer the explicit time endpoint for clock synchronization.

## Create, invite and join

1. Choose a movie or resolve the specific episode. POST the item and optional starting position.
2. Show a lobby with the readable eight-character code, optionally as `XXXX-XXXX`, and its expiry.
   Share it privately with household members or already signed-in friends.
3. The other device signs in normally and POSTs the code. Unknown/expired/restricted codes give
   generic recovery text; ask the host for a fresh code. New users still need Arc's normal friend
   invitation/server access first. A room code never grants library access or signs someone in.
4. Everyone opens the room stream, then prepares their own playback via preview/options/playback.
   Keep playback initially paused. Once primed, report ready with the current generation.
5. Any participant can press Play. If others aren't ready the room waits, then resumes together.

Rooms support both **codes and invitations to profiles**. Codes are useful when a TV is entering
a room; `/people` and `/rooms` support the invitation flow described below. Both allow late joins.
Codes last 15 minutes. A host can renew later without ending a long movie; the prior code stops
working. Room codes do not create accounts or grant access to media.

## Client sync algorithm

Use monotonic time for measuring RTT and local playback intervals; epoch milliseconds for the
server timestamp exchange. On entry/reconnect and periodically (for example every minute):

1. Save client epoch time C0 and call `/time?client_sent_ms=C0`; record C3 on receiving it.
2. With server receive S1 and send S2, estimate offset as `((S1-C0)+(S2-C3))/2` and network RTT as
   `(C3-C0)-(S2-S1)`. Prefer the lowest-RTT sample from a few requests. Clock asymmetry remains an
   uncertainty; refresh after sleep/network changes. No broadcast protocol guarantees frame lock.
3. Correct client wall time with that offset. When state is `playing`, expected file position is
   `position_ms + max(0, estimated_server_now - at_ms) * rate`. Otherwise use `position_ms` unchanged.
4. For absolute drift under 500 ms, normally do nothing. Between 500 ms and 3 seconds, use a small
   temporary rate correction (for example 0.97–1.03) **only if the player supports it**. Over 3 seconds,
   seek to the expected file position. If rate adjustment isn't safe, wait until drift warrants a
   seek rather than repeatedly interrupting playback. Revert to normal rate once caught up.
5. Room play/pause/seek comes from room commands, not from echoed local player events. Suppress
   event feedback while applying server changes. A local seek gesture sends one command after
   settling/debounce; it must not post a seek on every progress tick.
6. Send a heartbeat about every two seconds and immediately on entering/leaving buffering:
   current generation, **file** position, actual state (`playing`, `paused`, `waiting`) and ready.
   `waiting` means buffering and must have ready:false. A prepared paused player may be ready:true.
   These positions are diagnostic presence reports, not authority or ordinary watch progress.

**Converted-stream positions are always file positions.** Arc's `offset_ms` must be added to the
player's converted-stream clock. Seek using the client's existing PlaybackClock/seekInStream
logic: a position outside buffered/converted data requires a new personal playback request and
conversion session. Never send an HLS-relative currentTime as the room position. Audio/subtitle,
quality and conversion choices stay personal; reopening playback for a track change can make that
participant unready, but never changes the room's selected title or another viewer's settings.

Different editions/versions may have different cuts/timings despite sharing an Arc item identity.
Clients should prefer compatible editions and explain that sync is by file position, not semantic
scene alignment. Cross-edition, multi-part boundary and real-device synchronization are unverified.

## Buffering and timeout

A play, seek, title change or newly buffering participant can put a playing room into
`waiting`. Someone joining a room that is already playing does not: they load the title, seek to
the room's expected position themselves and catch up, as in Plex. Once they first report ready
they're part of the barrier like everyone else. The authority freezes its position and supplies wait_until_ms. Everyone pauses at that
position and displays who is not ready. Pausing explicitly keeps the room paused even if all are
ready. Once all *available* participants are ready, a room intending to play resumes.

A waiting episode lasts at most **10 seconds**. It then resumes and suppresses the still-slow
participants from that barrier until each reports ready again; repeated unready heartbeats cannot
stall the room indefinitely. A new seek/title creates a fresh generation and fresh barrier.
The slow participant catches up locally when playable. An unavailable member never holds up
others. If normal playback returns conversion ErrBusy, show/retry that participant's existing
busy error and report unready or leave; do not convert a personal failure into a room-wide error.
Other streams continue after the bounded wait. No room command bypasses transcode admission or
remote bandwidth/quality limits. New buffering waits and timeout policy are Arc's explicit rules;
Plex's public docs do not specify an exact buffering timeout.

## Live stream

Use a dedicated room stream rather than general `/events` invalidation plus GET: the existing
feed coalesces at 500 ms and only carries hints. Room updates wake immediately and carry complete
personalized state, eliminating a second request. Timeout transitions are checked every 200 ms.
That is a server-side delivery target, not an end-to-end WAN/device latency guarantee.

Web clients can use cookie EventSource. Native clients parse SSE over authenticated fetch.
Event name: `room`; data: room JSON; id: revision. Every connection starts with the **latest full
snapshot**; no replay history is required or retained. Last-Event-ID does not request old states.
Coalesced changes are intentional: adopt the newest revision, discard older ones. Periodic full
snapshots also refresh clocks/presence (same revision can appear again).

There is at most one stream per participant, independently of the general `/events` 32-stream cap.
Opening a new stream replaces that participant's older one, which then ends: a TV reconnecting after
sleep or a network change isn't locked out while the server still holds the old, half-open
connection. Clients should keep a single stream and simply reconnect when it ends unexpectedly.
No per-reader event queue or goroutine is spawned by the manager. Network writes happen outside
its mutex with a five-second deadline. Session/profile/access revalidation is at most two seconds
apart when writes are healthy; a stalled write can add its deadline. Stream transport expires at
30 minutes; reconnect with bounded backoff. SSE isn't a client heartbeat: continue readiness and
normal player progress reports. A connected stream maintains presence while its session remains
valid. On 401 sign in; 404 means membership ended/expired/restart, so leave the sync UI. On 429
respect Retry-After where supplied and back off. A stream sends ended then closes when the host
ends the room. Recheck `/current` or GET after a disconnect; do not guess or enumerate room IDs.

## Host departure, lifecycle and limits

- The host leaving, timing out or losing its session passes hosting to the oldest remaining
  participant. Other participants keep their individual playback sessions.
- A device with neither room heartbeats nor a healthy room stream loses membership after 30 s.
  A general room GET does not maintain presence. Rejoin with an unexpired code if needed.
- Empty rooms expire three minutes after the final member leaves/times out. With no readers,
  expiry/pruning is lazy on the next room API operation; deadlines still count from actual last
  presence and expired rooms cannot block new admissions indefinitely.
- Host End broadcasts ended; clients exit synchronized control and may offer Continue alone.
  It does not forcibly destroy individual playback or reset personal history.
- Ended-room tombstones last 30 s and count towards capacity. Restart clears every room; stale
  IDs return 404. Rooms/invites/messages are never persisted. No chat/reactions in this version.
- Limits: 20 rooms (including short tombstones), 10 participants each, one active room per
  device/profile, at most 200 room SSE streams, 10 commands/s and 4 heartbeats/s per participant.
- Five create/join attempts per minute/device via the existing auth limiter; five wrong or
  restricted codes per minute/address. A correct join clears the address's guess failures.
  Invite renewal shares the host's command rate limit. Payloads max 4 KiB. Positions 0–604800000 ms.
- Exceeding capacity/rates is explicit 429. Malformed input is 400; host-only actions 403;
  stale revision/generation 409; missing/nonmember/unplayable invitation 404. No silent drops.

The admin Dashboard's existing Playing records gain optional `watch_together_room_id` and
`watch_together_participants` for a matching participant/session/item. No additional room item,
member names or invitation codes are exposed to admins. Existing playback details remain as before.

## Client UX checklist

- Detail/episode action: Watch Together; lobby/create screen with expiring code/copy/share/renew.
- Join-code entry; recover own room after reload/profile/device changes without inheriting another.
- Participants, ready/waiting/unavailable indicators, host badge and "who paused" feedback.
- Shared controls with conflict recovery and no player-event feedback loop.
- Waiting overlay/countdown and catch-up treatment after a slow viewer times out.
- Personal playback settings, conversion-busy/error recovery and independent watch progress.
- Leave, host handoff, host End, lost connection/reconnect and server-restart handling.
- Dashboard badge/count, using the additive Playing fields; no room browser for admins.

## Logging, verification and reference

Info: created, joins/leaves, ended/expired, with opaque room ID, counts, peak, duration and fixed reason.
Debug: validated commands/revisions and waiting-state decisions. No per-heartbeat/position info,
invite codes, session tokens/hashes, messages or item titles. Ordinary input/access/rate/refusal
responses are expected 4xx, not new server warnings. Rooms/participants/rates bound event volume.
General change hints deliberately exclude all room routes so participation is not broadcast.

See the change note and PR for actual test commands/results. Unit/HTTP tests exercise revision
races, waiting/timeout, stale-generation rejection, expiry, host succession, room/participant/
stream/rate limits, library/age/label restrictions, friends, same-profile separate devices,
redacted stale responses, own-room recovery, SSE delivery and session revocation. They do not
establish actual multi-device sync, TV remote UX, WAN jitter, background/sleep handling, real
conversion saturation, HDR/audio output or sustained-load behavior. Those need client/device work.

Official reference only:
- [Plex Watch Together](https://support.plex.tv/articles/watch-together/): friends/content sharing,
  ready lobby and common controls. Plex says support is being retired from newer app experiences.
- [Plex Watch Together FAQ](https://support.plex.tv/articles/frequently-asked-questions-watch-together/):
  same-account multiple devices, individual streams/audio/subtitles, shared pause/seek, and
  invitation/rejoin behavior. The docs do not define an exact buffering or host-transfer algorithm.

The original code-first defaults, late-join, ten-second wait, host succession, friends default
and ephemeral-room choices are recorded on [#80](https://github.com/arc-media-server/arc-media-server/issues/80#issuecomment-6603360709) as deliberate defaults, not claims about Plex internals or parity. The Invitations section
describes the subsequently added profile-invitation flow.

## Invitations

Besides codes, a host can invite people by profile. `GET /people` lists who this
profile may invite: household profiles see every other profile (household and friends); friends
see only the household's admins, never its other profiles. With `item_id`, `can_watch` says whether
each may play that title; everyone invited must be able to.

`POST /rooms` makes a room for them with **nobody in it yet** (the creator included). It waits an
hour for people to join (a code-only room waits three minutes once empty), and one profile may
have three such rooms waiting. Each room lists on `GET /rooms` for its creator, the people invited
(until they decline) and anyone in it, newest first, leaving out rooms whose title the viewer may
not play. Listed rooms carry `id`, `state`, `position_ms`, `created_at_ms`, `host_name`, `hosting`
(this profile made it), `joined` (this device and profile are in it), the `item` summary, and the
names in `participants` and `invited`.

`POST /rooms/{id}/join` admits this device and profile without a code, after the usual access
check; whoever made the room becomes host when they join (the first person in holds it until
then). `POST /rooms/{id}/dismiss` declines an invitation, or, from the creator, ends the room for
everyone. The host can invite more people from inside the room (`POST /{id}/people`). Personalized
rooms gain `invited` (`[{name}]`): people invited who aren't in the room now.

A room list change (made, invited, joined, left, declined, ended) sends a personal
`watch_together` hint on `/api/v1/events` to everyone who sees that room, so Home rows refresh
without polling.

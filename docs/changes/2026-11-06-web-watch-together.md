# Web: Watch Together

The web app can now watch a movie or episode in step with other people, using the Watch Together
rooms Arc's server gained in #341 (docs/WATCH-TOGETHER-API.md). Everyone streams their own copy,
with their own quality, audio and subtitles; only the timeline is shared.

## What changed

- **Starting a room:** "Watch Together" in the ⋯ menu of a movie or episode. The menu is shared,
  so it's on title pages, episode rows and poster cards. Arc creates the room, and the player
  opens in it, paused at the start.
- **Inviting:** the player's people button shows who's in the room. For the host it also shows
  the invite code (XXXX-XXXX, click to copy), or "Get a new code" once it has run out
  (15 minutes).
- **Joining:** "Join Watch Together…" in the profile menu asks for the code. The player then
  opens on the room's title at the room's position.
- **In the room** (`components/Player.tsx`, `components/WatchTogether.tsx`):
  - Play, Pause and every seek (scrubber, keys, Skip Intro) go to the room, and every player
    follows it. The player that seeks moves at once rather than waiting for the room's answer,
    so repeated key presses add up.
  - Players stay within half a second of where the room should be. A drift under 3 s is fixed
    by playing at 97% or 103% speed until caught up; a bigger one by jumping. The playback speed
    menu is hidden, since the room plays at normal speed.
  - "Waiting for Sam…" shows while someone's player catches up, with the seconds left: Arc
    carries on after 10.
  - "Paused by Jo", "Played by Jo" and "Jo moved the room" show when someone else does it.
  - Someone who can't watch the room's title is told so and can stay or leave.
  - Up Next belongs to the host: when the host's episode ends, the room moves to the next one
    and everyone's player follows.
- **Leaving:** closing the player leaves the room; "Leave" in the people menu does the same.
  The host can "End for everyone". Then, and if the room is lost (a server restart), the player
  carries on alone and says so.
- **After a reload,** a banner offers to rejoin the room this device is still in, or leave it.
- **Dashboard:** a stream in a room shows "Watch Together · 3 people".
- `lib/watch-together.ts` (no React):
  - the room API, where a conflict returns the current room;
  - the room's live stream, with reconnects;
  - heartbeats every 2 s;
  - clock sync (three `/time` samples, the shortest round trip wins, again every minute);
  - the expected-position and drift-correction maths.

## Decisions (logged on #80)

- **The entry point is the ⋯ menu,** as in Plex, not a new button beside Play.
- **The room starts at the beginning,** not at the host's resume point. Anyone can seek once
  everyone's in.
- **Only the host's Up Next moves the room on.** Everyone's player would otherwise try to change
  the title at the same moment.
- **Joining is by code only** (the server's design): no list of rooms or of people to invite.

## Checks

- `tools/watch-together.test.mjs` (in `npm run build`): codes, clock offset (the
  shortest-round-trip sample wins), the expected position while playing, paused and waiting,
  and the drift thresholds.
- Browser journey `e2e/watch-together.spec.ts`: two separate signed-in browser contexts. The host
  starts a room from the ⋯ menu and reads the code; the second joins from the profile menu. Both
  wait paused, then both play when the host presses Play. The guest pauses and the host's player
  pauses, with "Paused by…". The host seeks 20 s and the guest's player moves there. The guest
  closes the player and the host's count drops to one.
- Not checked: real networks between homes, phones, Safari's player, or casting from a room.

## Logging

Nothing new on the server. The web app logs nothing per beat. Room refusals show in the browser's
own network tools.

## Limits

- Casting to a Chromecast from a room plays outside the room (the cast device isn't a
  participant).
- The Android TV and Apple TV apps don't have Watch Together yet.
- A participant whose conversion is refused (the server's limit of conversions) sees the usual
  error; the room carries on without them.

# Watch Together: invite people, and rooms on Home

Watch Together now works by invitation. The host picks who to watch with, presses Done, and
the room appears on everyone's Home in a Watch Together row; they join from there. Codes and
invite links still work alongside.

## What changed

### Server (additive `/api/v1`)

- `GET /watch-together/people?item_id=`: who this profile may invite, with `can_watch` for the
  title. Household profiles see everyone else; friends see only the household's admins, never its
  other profiles.
- `POST /watch-together/rooms {item_id, profile_ids}`: a room for the chosen people with nobody in
  it yet. It waits an hour for them (a code-only room still goes three minutes after emptying).
  A profile may have three such rooms waiting.
- `GET /watch-together/rooms`: the rooms this profile made, was invited to or is in, newest first.
  Rooms whose title it may not play are left out.
- `POST /watch-together/rooms/{id}/join`: join from the list, without a code.
  - The creator is host whenever they join.
  - Until the creator joins, the room has no host, and the people invited wait for them.
- `POST /watch-together/rooms/{id}/dismiss`: decline, or cancel the room (from its creator).
- `POST /watch-together/{id}/people`: the host invites more people from inside the room.
- Rooms gain `host_name` and `invited`.
- Any change to a room's list (made, invited, joined, left, declined, ended) sends a personal
  `watch_together` hint on `/api/v1/events`, so Home rows update live.

### Web

- **Choosing people:** ⋯ → Watch Together opens "Choose who to watch … with".
  - It lists household profiles and friends; anyone who can't watch the title is greyed out.
  - Done makes the room and says it's on Home, with "Join now".
  - "Use a code instead" makes a code room and opens the lobby, as before.
- **Home's Watch Together row:** shown only when there are rooms. Each card shows the title's
  artwork, "Hosted by Jo" or "You're hosting", and who's there or invited.
  - Buttons: Join (Return when already in it), then Decline, or Cancel for the host.
- **Lobby:** lists invited people as "Invited" until they arrive. The host has an Invite button
  for more people. The code and link are tucked under "Share a code or link instead".

## Decisions (logged on #80)

- **Host:** a room made by invitation waits for its creator to start it.
- **Lifetime:** a room waits an hour for people to join.
- **Friends:** friends can invite only the household's admins.
- **Home row:** it appears only when there are rooms, at the top of Home, and isn't in Edit Home.

## Checks

- Go:
  - `internal/watchtogether/invited_test.go` covers lists and strangers, the title check, the hour
    wait, who becomes host, decline, cancel, the per-profile limit, and inviting more people.
  - `internal/api/watch_together_rooms_test.go` covers who can be invited (friends don't see the
    household), refused invitations, live hints to every invitee, lists, joining, restricted
    titles dropping out, decline, invite more, guests not inviting, and cancel.
- Browser, `e2e/watch-together.spec.ts`, two signed-in devices:
  - Sam's Home is already open when the host invites Sam from ⋯, and the room appears there
    without a reload;
  - the host's Home shows "You're hosting";
  - Sam joins and waits for the host, and the host's card shows "Sam is here";
  - the host joins and starts, then play, pause and seek stay in step;
  - Sam leaves, then the host cancels and the room disappears from Sam's Home.
  - A second journey checks the code and invite link through the lobby.

## Logging

- The server logs room creation ("created with invitations"), joins, leaves, declines and ends at
  info, with the room ID, counts and duration, never names or codes.
- Live hints carry only the topic.

## Limits

- The Android TV and Apple TV apps don't show the row or the invite picker yet (the API is ready).
- No push notifications outside Arc: an invitation shows when the person opens Arc.

## Also fixed: live updates refreshed everything on every page load

A new events stream has no Last-Event-ID, so the server opens it with `resync`. The web app took
that as "refresh everything" about 1.5 s after each load, which wasted requests and refetched
panels mid-way through being used. It broke the Settings recovery journeys (recovery:187, all
three panels). The first `resync` of a fresh stream is now skipped. A resync after the browser
reconnects the same stream, or the refresh after reopening a stream that was away, still refreshes.

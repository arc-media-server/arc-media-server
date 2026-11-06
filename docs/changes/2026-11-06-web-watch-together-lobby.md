# Web: Watch Together lobby and invite links

Starting Watch Together opened the player paused, and the invite code was hidden in the small
people button at the top of the player. It looked as if nothing had happened and there was no way
to invite anyone.

## What changed

- **Lobby:** until a room first plays, the player shows a card over the paused video.
  - The host sees the invite code (click to copy), "Copy invite link", who's here with their
    state, and **Start** and Leave.
  - Everyone else sees who's here and "Waiting for Jo to start…", with Leave.
  - Start (or Play) begins the room for everyone and the lobby closes. Someone who joins a room
    already under way goes straight to the video.
- **Invite links:** `https://<server>/?together=CODE` opens Join Watch Together with the code
  filled in. The code is removed from the address once read. Signing in comes first, as for any
  page.
- The people menu in the player still has the code, now with the link as well; the lobby and the
  menu share the same participant list and invite pieces.
- The main buttons (Start, Join, Rejoin) use the primary style.

## Decisions

- **The host starts the room,** rather than it starting by itself
  when someone joins.
- **Links fill in the code but don't join on their own:** one click on Join, so a link opened by
  mistake doesn't put anyone in a room.

## Checks

- `e2e/watch-together.spec.ts` now covers the lobby and the link:
  - the host's lobby shows the code and the link;
  - the guest opens the invite link and the code is filled in;
  - the guest's lobby waits for the host, and the host's lobby shows 2 people;
  - Start closes both lobbies and both players play;
  - pause, seek and leave are checked as before.
- The ⋯ menu journeys and live updates still pass.

## Logging

Nothing new: a web-only change. Room refusals show to the user and in the browser's network tools.

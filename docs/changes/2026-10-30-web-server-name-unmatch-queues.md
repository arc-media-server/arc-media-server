# Web: server name, Unmatch and server play queues

The web side of the backend handoff (docs/BACKEND-API-HANDOFF.md): the server's friendly name,
Unmatch for shows, and the player's queue kept on the server.

## What changed

- **Server name.** Settings → Server starts with a General section and a Server name field (up to
  80 characters, saved with `PATCH /settings {"server_name"}`). Apps show this name (identity).
- **Unmatch.** A matched show's ⋯ menu (admins, not Other videos, not already unmatched) has
  Unmatch…, with a confirmation that says what resets (match, artwork, episode list, orderings)
  and what stays (files, watch history, edits to the title). It calls `POST /items/{id}/unmatch`
  and refreshes the show, its seasons and the library lists.
- **Play queues.** When `/capabilities` says `play_queues` is available and allowed, the player's
  queue is mirrored to the server queue for this device and profile (`web/src/lib/play-queue.ts`):
  - starting a title, an episode list or a playlist creates the queue (replacing the old one);
  - Play Next, Add to Queue, move, remove, Clear upcoming and Auto-Play's next episode are the
    matching revision-checked edits;
  - moving on (an item ending, Next, Previous, Play now) advances or selects;
  - closing the player deletes the queue. Leaving the profile keeps it.
  - After a reload, a small "Pick up your queue" card offers Resume or Dismiss.
  - If an edit is refused (another tab with the same sign-in changed it, or it expired), the
    server's queue is read back. The current title keeps playing, the upcoming list follows the
    server, and the queue dialog says so.
  - Unavailable entries are listed dimmed (they can be removed) and skipped.
  - Without the capability, or for lists the server can't hold (trailers and extras, more than
    1000 titles), the queue stays in the page as before.
  - Pre-roll videos stay local; they aren't titles in the server's queue.

## Decisions (logged on #80)

- Closing the player is a deliberate stop and discards the server queue.
  A reload or a closed tab keeps it, and the app offers to resume it instead of starting
  playback on its own.
- The queue stays the player's own list: the server copy follows it. A full rebuild from the server
  happens only on recovery or a refused edit, so resume points, editions and preplay choices
  chosen in this page still apply.

## Limits

- Two tabs sharing a sign-in share the queue but don't update each other live. A tab sees the
  other's edits on its next refused edit or reload.
- Shuffle and unshuffle aren't used by the web yet: Shuffle builds the shuffled list before the
  queue is created.

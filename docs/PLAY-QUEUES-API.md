# Server play queues

All routes are under `/api/v1`, require an authenticated selected profile, and accept
Bearer authentication or the existing cookie. **Use the same device session token after app restart**
to recover that device's queue. Another login/token is another device/session. Two clients deliberately
sharing a token share its queue. Switching profiles keeps separate queues. Revoking a session removes
its queues. No administrator override exposes another device/profile's queue.

## Lifecycle and contract

- `POST /play-queues` creates a queue: `{"item_ids":["<movie-or-episode-or-show-id>"]}` or
  `{"playlist_id":"<own-playlist-id>"}`. Returns 201 and the complete queue.
- A show expands to its available episodes in Arc's existing display/specials order, with unplaced
  episodes retained. Playlist contents are copied at creation, including duplicate entries. Later
  playlist edits do not alter the queue. Existing smart-playlist limits still govern that source.
- To replace an existing queue, include `replace_id` and its current `revision` alongside the source.
  Without them replacement returns 409. An expired queue is absent; create without replacement fields.
- `GET /play-queues/current` recovers this session/profile's queue; 404 means none/expired.
- `GET /play-queues/{id}` retrieves a specific owned queue. Other device/profile/unknown IDs all 404.
- `DELETE /play-queues/{id}?revision=<number>` removes it, returning 204. It does not stop a video,
  transcoder or cast receiver; the client stops its own playback through the existing flow.

Response (illustrative):

```json
{
  "id":"<queue-id>","revision":1,"current_entry_id":"<entry-id>",
  "shuffled":false,"expires_at":1794000000,
  "entries":[
    {"id":"<entry-id>","available":true,"item_id":"<arc-id>","item":{"id":"<arc-id>","title":"..."}}
  ]
}
```

Item is the existing item-list DTO with profile watch data; the snippet is abbreviated. Entry ID and
item ID are different: the same movie can occur twice with two distinct entry IDs. Use **entry IDs**
for removal, ordering and selection. Queue IDs and entry IDs are opaque. Responses are private/no-store.

Limits: 1000 expanded entries, 128 KiB request body. Exceeding the cap is rejected, not silently
truncated. Queue metadata/entries are read in one database snapshot. Revisions increase atomically
with mutations. Stored queues survive normal server/app restart and database backup/restore. After
seven days without access/mutation they expire; successful reads extend the lease at most hourly,
so normal polling does not cause a database write per poll. Expired rows are pruned on queue creation.

An entry whose file is missing or whose title is no longer permitted becomes a placeholder:
`{"id":"<entry-id>","available":false}`. No item ID, title, artwork or other metadata is returned.
Claude: show an unavailable entry and allow skip/remove. Never try to infer hidden metadata from the
queue. Playback endpoints independently enforce permissions and availability.

## Editing

`PATCH /play-queues/{id}` always includes the latest `revision` and one `operation`:

| Operation | Additional fields | Behavior |
|---|---|---|
| `append` | `item_ids` | Add at the end; shows expand as above |
| `next` | `item_ids` | Insert immediately after the current entry |
| `remove` | `entry_ids` | Remove exact entries; current entry cannot be removed |
| `reorder` | `entry_ids` containing every entry exactly once | Reorder upcoming entries; current/past prefix must stay unchanged |
| `clear` | none | Remove upcoming entries, retaining current and past entries |
| `advance` | none | Move to the next entry; empty current_entry_id means finished |
| `select` | `entry_id` | Set queue position explicitly (e.g. Previous or selecting an entry) |
| `shuffle` | none | Shuffle only upcoming entries, preserving current/past |
| `unshuffle` | none | Restore the original order of remaining upcoming entries |

Mutations return 200 with the updated queue. The queue's membership/order before a shuffle is its
baseline; newly added entries during shuffle append to that baseline. Explicit reorder establishes a
new unshuffled order. Past entries remain in the snapshot and count toward the cap; start a new queue
when a long-lived session reaches the cap.

A stale revision returns 409 without applying the mutation. Reload and let the user retry/reconcile;
do not blindly overwrite. A stale queue ID cannot accidentally target its replacement. Unknown or
foreign entry IDs, duplicates in reorder/removal, unsupported operations and wrong extra operation
fields return 400. Hidden/unknown/unavailable input media return the same 404. Malformed input returns
400; ordinary storage failures use the existing error envelope. Permission checks apply on every read.

## Client integration

1. Detect `play_queues` in `/capabilities`. The web app already uses these endpoints; native clients should detect support on their server.
2. On app start/profile selection, GET current. Bind cached state to server ID/profile/device token.
3. When starting a title or playlist, create/replace the queue and use the returned current entry.
   These calls **do not start playback**. Continue using playback-options/playback for file, edition,
   audio/subtitle choice and resume. The queue stores logical titles, not a frozen file/track choice.
4. Replace local Play Next/Add/remove/reorder/clear logic with the revision-checked operations. Keep
   existing shared controls and optimistic rollback; handle 409 with a reload/reconcile message.
5. On actual playback completion, advance once using the current revision, then start the next
   available entry. Select updates queue position for previous/jump; it does not seek/start playback.
6. Explicit Stop should stop the current player and use queue deletion if stopping should discard it.
   A temporary disconnect/app close should retain the token and queue for recovery.

No phone-as-TV-remote, automatic handoff, synchronized playback, repeat policy or shared cross-device
household queue is introduced. Those need separate app/server contracts. Queue mutations themselves do not start or stop a player.
Progress/resume and watched state remain on existing endpoints; queue advance does not mark watched.

## Logging and verification boundary

Creation logs one outcome with queue ID and entry count. Edits/polls do not add info-level lines;
storage errors use existing API failure logging. No session hashes/tokens, item paths or request
bodies are logged. Bounds and one statement snapshot avoid unbounded/partial queue responses.

Tests use generated database records and temporary stores. They verify duplicate entry IDs,
Play Next/current protection, shuffle/unshuffle, stale concurrent edits, rollback, exact reordering,
expiry, backup/reopen persistence, device/profile isolation, session revocation, playlist snapshots,
changed-access redaction and protection against episode cleanup/Unmatch. Real player/UI/native-device
behavior remains Claude/owner acceptance.

Migration 0062 introduced the queue tables. The API adds fields/routes without removing existing v1 behavior.

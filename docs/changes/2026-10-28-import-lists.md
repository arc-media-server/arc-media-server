# Import watchlists, playlists and collections (#80 item 30, 28 October)

The Plex and Jellyfin import now also brings lists, after each account's watch state:

- **Plex:** the signed-in account's plex.tv **watchlist** (read from discover.provider.plex.tv
  with the sign-in token; `AMS_PLEX_DISCOVER_URL` points it at a stand-in) and its **video
  playlists**. Plex only shows these to the account itself, so other people's aren't read. The
  server's **collections** (movie and TV libraries) come over for everyone.
- **Jellyfin:** each user's **favourite** movies and shows become their watchlist, their own
  **playlists** come over, and the server's **collections** (box sets, as the administrator sees
  them).
- Titles are matched by provider ids (shows by TheTVDB/TMDB/IMDb ids; episodes as before). A
  playlist keeps its order. A collection goes into the Arc library its titles are in (one per
  library if they span several), as a manual collection. Smart playlists and collections come over
  as fixed lists of what they hold now.
- Importing again adds only what's missing: the profile's playlist or the library's manual
  collection of the same name is reused, and its existing entries stay where they are.
- Titles Arc doesn't have are counted and listed (with the collection's name for collections).
- If lists can't be read, the watch state still comes over and the results say why
  (`lists_error` per account, `collections.error`).

The import results (`GET /api/imports/{id}`) gain per-account `watchlist`, `playlists`,
`playlist_items`, `lists_not_found`, `lists_error`, and a `collections` summary. The Moving in
dialog shows them: each person's watchlist and playlist counts under their watch-state line, a
Collections entry, "not in Arc" lists for both, and any list error. Settings' Plex/Jellyfin rows
and the dialog's description now mention watchlists, playlists and collections.

Not tested against real Plex/plex.tv/Jellyfin servers: covered by stand-in servers (the
watchlist endpoint and response shape follow Plex's documented discover API but couldn't be
fetched from here).

Checks: go vet/test (new tests: Plex watchlist/playlists/collections incl. order, not-found
listing, owner-only lists and re-import without duplicates; Jellyfin favourites/playlists/box
sets; list failures keep watch state). Playwright: a journey shows a finished import's watchlist, playlist and collection
counts and both "not in Arc" lists (the import itself is stubbed; it needs a real server).

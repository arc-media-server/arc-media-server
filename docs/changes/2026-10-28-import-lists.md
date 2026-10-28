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
dialog doesn't show these new counts yet: ImportDialog is in an open translation PR (#247), so
that small UI change follows its merge. The lists themselves appear in Arc straight away.

Not tested against real Plex/plex.tv/Jellyfin servers: covered by stand-in servers (the
watchlist endpoint and response shape follow Plex's documented discover API but couldn't be
fetched from here).

Checks: go vet/test (new tests: Plex watchlist/playlists/collections incl. order, not-found
listing, owner-only lists and re-import without duplicates; Jellyfin favourites/playlists/box
sets; list failures keep watch state).

# PostgreSQL download and import coverage

[#308](https://github.com/arc-media-server/arc-media-server/issues/308)

Add 14 existing scenarios to the PostgreSQL release gate. They also run on
SQLite in the default Go suite.

- Downloads cover concurrent preparation requests and device limits,
  interrupted preparation and database reopen, bundle cleanup, HTTP/1.1 and
  HTTP/2 range delivery, and revocation during a transfer.
- Local Plex and Jellyfin fixtures cover watch history, resume progress,
  Plex ratings, playlists, watchlists and collections. The Plex tests check
  that reimporting adds no duplicate history or list entries. A list failure
  still preserves imported watch state.

The PostgreSQL replay found no engine-specific failures in these scenarios.
This change affects test fixtures and release verification. It adds no runtime
logging and requires no product decision.

## Validation

All 60 shared scenarios passed in the disposable PostgreSQL 19 race gate,
including the 14 added here. The gate also passed its PostgreSQL startup,
upgrade and recovery tests. The default SQLite Go suite, vet, web typecheck,
Vite production build and embedded server build were run locally. The embedded
build reused assets from the same web source tree.

The tests used synthetic media bytes and local import fixtures. No live Plex
or Jellyfin server, owner data, browser journey or published Docker image was
used for this batch.

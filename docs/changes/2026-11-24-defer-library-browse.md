# Load the library grid and filters when needed

The Recommended tab loads its own rows. Arc fetches the Browse grid and filters when the
Library tab opens, and collections when their tab opens or when Browse needs cards for its
unfiltered grid. The Collections tab is visible to every profile with access to the library and
uses its empty state when none are visible. These results stay in the usual query cache.
Letter jumps and scroll restoration run while the Browse grid is visible.

On a 10,000-movie fixture, simulating the reduced request group lowered median Recommended
handler time from 474 to 293 ms on SQLite and from 461 to 286 ms on PostgreSQL 19. These are
server-side measurements. The browser journey checks which requests actually start and
that collections, titles and video filters work after switching tabs. That fixture kept
Collections in both timing groups; the final tab scheduling defers Collections as well.

Browse request failures still show on the Library tab, with its existing retry controls.
Existing HTTP logging covers the requests.

A second paired trial isolated Collections. Recommended alone took a median 190 ms on SQLite
and 271 ms on PostgreSQL 19, down from 317 ms and 295 ms when Collections ran alongside it.

# Load the library grid and filters when needed

The Recommended tab loads its own rows and collections. Arc fetches the Browse grid and
filter options when the Library tab opens, and keeps them in the usual query cache. It also
waits until the grid is shown before handling letter jumps and scroll restoration.

On a 10,000-movie fixture, simulating the reduced request group lowered median Recommended
handler time from 474 to 293 ms on SQLite and from 461 to 286 ms on PostgreSQL 19. These are
server-side measurements. The browser journey checks which requests actually start and
that titles and video filters work after switching tabs.

Browse request failures still show on the Library tab, with its existing retry controls.
Existing HTTP logging covers the requests.

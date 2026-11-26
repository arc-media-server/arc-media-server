# Resurface replaced movies in Recently Added

When a validated scan finds a new file replacing the only available source for
an existing movie, that movie moves up in Recently Added on Home and in the
library. Arc keeps the original arrival date, title ID, watch state and history.

The scanner looks for a different-sized file first discovered after the old
source was last seen. Both files must match the same movie and be in the same
folder. The old file must be present in the catalog immediately before missing
file reconciliation. Scans retain the usual storage-availability checks.
Both the missing-file flag and the new Recently Added position commit together.

This covers the common upgrade where a media manager swaps one release filename
for another. Same-path changes, same-size changes, cross-folder moves, multipart
movies, additions that leave an alternate source present, and old versions
already marked missing do not count. The migration does not promote existing
movies on its own. Arc uses the same rule on SQLite and PostgreSQL.

Plex can resurface an existing movie after a file upgrade when its scanner treats
the replacement as a new entry. Arc moves the existing title up in the list.
[Plex upgrade report](https://forums.plex.tv/t/replacing-existing-movie-results-in-a-recently-added-movies-entry-or-new-movie-notification/937990)

## Logging

The scan completion log includes an aggregate count of movies moved up in
Recently Added. No new per-file logs.

# Merging TV shows (#80 item 27, 28 October)

`POST /api/items/merge` now merges shows as well as movies (same library, same kind). Typical
case: two folders of one show ("Doctor Who (2005)" and "Doctor Who") that became two shows.

- The kept show keeps its details, display orders, placements and artwork.
- Each merged show's episodes join it. An episode both have (same episode key) gives the kept
  episode its files; per-profile watch state is combined keeping the most progress (watched wins,
  then play count, then the latest resume point), and history, playlist/collection entries and
  labels move. An episode only the merged show had moves across as it is (same id), so its
  history, resume point, markers and edits stay.
- Show-level watchlist, rating, labels, playlist/collection entries, extras and history move.
- The merged show's folder key becomes an alias of the kept show (migration 0057,
  `item_aliases`), so later scans file that folder's episodes under the kept show. Merging the
  kept show into another later carries its aliases along. Scans lock merged folders as one show.
- The merge runs under the library's work slot, then refreshes the kept show and re-links all
  its files to its episodes (as Refresh metadata). Busy library: 409, nothing changes. If only the
  refresh fails, the merge stays and the error says to use Refresh metadata.
- Shows that number their files differently (TheTVDB vs TMDB File episode order) are refused
  with a message to set both the same first, since their episode keys can't be paired.
- The merged show's own display orders and manual placements are dropped (the kept show's stay).

In the app: Library Health's Duplicates list now offers Merge for shows as well as movies, and
so does the selection bar in a TV library (Select, then Merge). The Merge dialog explains what
happens to a show's episodes and orders, and that shows can't be split apart again. Both the
Health list and the dialog now show where each copy is (a movie's file, or a show's folder), so
two "Arc Show (2020)" entries can be told apart (`folders` in the library health response).

Checks: go vet/test (new store test: shared/own episodes, progress combining, show-level marks,
history, aliases and alias carry-over); Playwright: new journey against the real scanner (a
second folder with a shared and an own episode, merge, full rescan keeps one show with the
same episode ids, progress and both files), plus maintenance, episode-details and clarity specs.
A second journey merges from Library Health: the dialog names both folders, and afterwards one
show remains and the Duplicates entry is gone.

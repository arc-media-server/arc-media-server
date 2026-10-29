# A fuller ⋯ menu on each library page

The owner showed Plex's library ⋯ menu (Unpin, Scan Library Files, Grant Access…, Manage
Library ▸ Edit…, Refresh All Metadata, Empty Trash, Analyse, Delete) and asked for the same on
Arc's library page, next to the filters, except Delete.

What changed:
- **The library page's ⋯ (server owner only) now offers:** Select titles (Titles tab only), Scan
  library files, then a separator and Edit library…, Refresh all metadata… (not for Other videos
  libraries, which are never looked up online), Reanalyze files…, Empty trash… and Manage access….
  Refresh and Reanalyze ask first and Empty trash opens its dialog, exactly as on Manage Libraries.
  Edit opens the same library editor as Manage Libraries' Edit button.
- **The ⋯ is on every tab** (Movies/Shows/Videos, Recommended, Folders, Collections), not only
  Titles. On Titles it stays at the right of the sort and filter bar; on the other tabs it sits in
  the same spot, right-aligned at the top of the content (beside New Collection on Collections). A
  running scan's progress shows next to it on every tab.
- **One implementation for both menus.** The refresh/reanalyze start, their confirmation and the
  Empty trash dialog moved out of `LibraryWorkControls` into `useLibraryWork`,
  `LibraryWorkItems` and `LibraryWorkDialogs` (`web/src/components/LibraryWork.tsx`). Manage
  Libraries' menu uses them too and is unchanged, including Stop and Remove library….

Decisions:
- **No Delete/Remove** on the library page, as the owner asked. Removing a library stays on
  Manage Libraries, behind its confirmation.
- **No Unpin.** Arc has no pinning: every library is in the sidebar.
- **Manage access… is included and opens Profiles.** Plex's Grant Access… shares the library with
  people. In Arc, which libraries a profile or friend can see is set per profile (Profiles, Edit,
  Libraries), and the Profiles page lists each profile's and friend's library count and has Add
  friend ("Share libraries with someone"), so it is the right place to send the owner. There is no
  per-library "who can see this" view yet; if one is added, this item should open it instead.
- Wording follows Arc's existing sentence case and Manage Libraries' labels ("Reanalyze files…"
  for Plex's Analyse, "Empty trash…"); Edit library… and Manage access… are the two new strings.

Limits: Stop for running work stays on Manage Libraries and in Activity; the library page shows
progress only, as before.

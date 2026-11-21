# Library page: a shorter bar, an A–Z rail and poster hover actions

The owner found the library's top bar crowded (sort, Jump to, Reverse, Select, Scan Library, five
filter chips and a full-width "More filters" panel). Looking at other media servers, it now has one
short row of controls, an A–Z rail and Plex-style actions on each poster.

What changed:
- **Views** (Movies / Recommended / Collections) are one pill switch at the top right.
- **Sort and direction** are two small menus, [Title ▾] [A–Z ▾]. The direction is named for the
  sort (A–Z, Newest first, Highest first…); the separate Reverse button is gone.
- **Filters** is one button opening a panel with every filter (unwatched, genre, label, language,
  decade, quality, added, length, rating). The ones that are on show beside it as chips that
  remove themselves, and the button counts them.
- **A–Z rail** down the right of the grid when sorted by title, as in Plex: choosing a letter
  scrolls the whole list to its first title, loading the pages up to it first (titles before it
  stay). Letters with no titles are dimmed, and the rail follows scrolling. The old "Jump to"
  menu showed a list starting at the letter and hid the titles before it. Links with `?letter=`
  still work: they jump, then drop the letter from the address.
- **Poster hover actions** (as in Plex): a small circle to start choosing titles with that one
  chosen (server owner), Play in the middle (a show plays its next episode), the pencil to edit
  (server owner) and "⋯" with the title page's menu plus Mark as Watched and Watchlist. Right-
  click opens the same menu. Touch screens, which can't hover, keep the plain poster; "Select
  titles" is also in the library's ⋯ menu. Watchlist and Search posters get Play and ⋯ too.
- **Scan Library** moved into the library's ⋯ menu ("Scan library files", as Plex's library
  menu): Arc scans by itself when folders change and hourly. A running scan still shows its
  progress in the bar.
- API: `GET /api/items?letters=1` adds `letters`, the filtered titles counted by first letter in
  title order, so the rail knows where each letter starts (`store.TitleLetters`).
- Fixed: the API docs generator lost a dozen route descriptions when the README's API table was
  removed (#262), failing `TestDocsUpToDate` on main. Those handlers now carry the descriptions
  in their doc comments.

Decisions (logged on #80): selection starts from a poster's circle (as in Plex) or the library ⋯
menu; there's no Select button in the bar. Results of a poster menu
action (queued, refreshing…) show as a short note at the bottom of the screen, since a poster has
no room for them.

Limits: posters inside a collection (collection display mode) have no card of their own, so a
jump lands on the next title that has one. Accented first letters (É) count under "#".

# Solid menus, and clearer ordering, specials and skip labels

From the owner's look at the poster ⋯ menu and the Edit dialog, plus a review for more of the same.

What changed:
- **Menus are solid.** Dropdown menus used the frosted "glass" style (72% opaque with a blur that
  didn't take effect over posters), so their text was hard to read. They now use the solid surface
  the Activity, News and Filters panels already use.
- **Episode ordering… is no longer in the server owner's ⋯ menu** (title page and posters): the
  owner sets it in Edit, Metadata & ordering. Profiles without Edit keep it in ⋯, for their own
  order (see the review list below).
- **Display order's default names the order it follows**: "Match file episode order (TheTVDB
  aired order)" instead of "Follow file order". It follows the File episode order chosen above it,
  including a change not yet saved.
- **Specials in seasons is On / Off for the server owner.** "Default (Off)" and "Off" did the same
  thing for the owner. Profiles keep a third choice, "Use server setting", which follows the
  owner's.
- **Library settings say "Offer Skip Intro" and "Offer Skip Credits"** (were "Skip intros" and
  "Skip credits"): they decide whether Arc offers the button, while the profile's "Skip intros
  automatically" decides whether it's pressed for you. The near-identical labels read like the
  same setting twice.
- **Settings, Your profile: "On this page" lists Home** (it listed Playback, Subtitles and Display
  only).
- **Edit, Metadata & ordering explains itself in plain words**: "The show's details come from the
  library's metadata provider, TMDB. Episode titles, descriptions and images come from the file
  episode order below."

Reviewed and left as they are: a collection's "As the library is set (…)" (it follows the
library's setting if that changes later, as Plex's "Library default" does) and "Mark as watched
at: 90% (default)".

## What other profiles can change (for the owner's review; nothing changed)

Checked against the server, not just the buttons: a profile without admin rights is refused
(403) every change to a title for everyone (edit, refresh, analyze, artwork, Fix Match, labels,
seasons, collections, merge, split, markers, scan, forget). Their own episode order and specials
choices apply to them alone (checked: the owner's view stayed the same). Listed on #80:

1. **Episode order and specials per profile.** Any profile can choose its own display order and
   "Specials in seasons" for a show (⋯, Episode ordering…). Only theirs changes. Plex doesn't
   offer this to other users.
2. **Subtitle downloads.** Any profile, including friends, can download a subtitle from
   OpenSubtitles. It's kept with the file for everyone and uses the server's OpenSubtitles
   allowance.

Everything else a profile can change is its own: watched state, progress, ratings, watchlist,
Continue Watching, playlists (owner-checked), preferences, and stopping its own conversions.

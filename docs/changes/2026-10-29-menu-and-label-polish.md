# Solid menus, and clearer ordering, specials and skip labels

From the owner's look at the poster ⋯ menu and the Edit dialog, plus a review for more of the same.

What changed:
- **Menus are solid.** Dropdown menus used the frosted "glass" style (72% opaque with a blur that
  didn't take effect over posters), so their text was hard to read. They now use the solid surface
  the Activity, News and Filters panels already use.
- **Episode ordering… is no longer in the ⋯ menu** (title page and posters): it's in Edit,
  Metadata & ordering, for the server owner only (below).
- **Display order's default says what it is**: "Library default (TheTVDB aired order)" instead of
  "Follow file order", or "This show’s file episode order (…)" when the show's File episode order
  is changed above it (including a change not yet saved).
- **Specials in seasons is On / Off.** "Default (Off)" and "Off" did the same thing.
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

## What other profiles can change

Checked against the server, not just the buttons: a profile without admin rights is refused
(403) every change to a title for everyone (edit, refresh, analyze, artwork, Fix Match, labels,
seasons, collections, merge, split, markers, scan, forget). The review found two more, and the
owner decided both are the server owner's (29 October):

1. **Episode order and specials are the server owner's.** Profiles could choose their own display
   order and "Specials in seasons" for a show. Now every profile sees the owner's choice: the
   server refuses (403) other profiles' changes and the option is gone from their menus. Profiles'
   earlier choices are kept in the database but no longer read, so this is easy to undo. Plex
   doesn't offer this to other users either.
2. **Subtitle downloads are the server owner's.** A download is kept with the file for everyone
   and uses the server's OpenSubtitles allowance, so Search and download are admin-only and
   "Find Online…" is hidden in other profiles' players. They still pick from the subtitles there
   are.

Everything else a profile can change is its own and stays open: watched state, progress, ratings,
watchlist, Continue Watching, Home rows, playlists (owner-checked), preferences, and stopping its
own conversions.

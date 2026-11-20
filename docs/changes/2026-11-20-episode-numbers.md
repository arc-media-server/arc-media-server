# Episode numbers read "S1 · E4" everywhere

The apps wrote the same episode several ways: "S1 E4", "S1 · E4", "S01 · E04", "S1, E4 • title",
and sometimes put the episode title on a card's line and sometimes not. The owner chose
"S1 · E4" (20 November); the rule is now in `docs/AGENT-RULES.md` ("Episode numbers") for every app.

This PR brings the server and web app in line:

- `episodeCode()` (cards, the player's line, episode menus), the Order comparison, the Dashboard's
  now-playing line, Library Health's missing-episode ranges, the playlist entries and the server's
  activity labels ("Grey's Anatomy · S4 · E12") read "S1 · E4".
- Home's Recently Added captions drop the leading zeros ("S1 · E1", was "S01 · E01"), and a
  special reads "Special 3" (was "S00 · E03").
- Episode cards (Continue Watching, Recently Watched Episodes, other episode rows) show the
  show's name and then just "S1 · E4"; the episode title no longer shares that line. Recently
  Added keeps its own title line.

Decisions: Plex writes "S1 · E4" on its cards and leaves the episode title off them, which is
what this matches. The alternative was "S1 E4", shorter but a different style from Plex.

Checks: see the PR. Logging: none needed (labels only; the activity label is unchanged apart
from its wording).

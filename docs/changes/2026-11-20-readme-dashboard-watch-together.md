# README: Dashboard and Watch Together pictures

The README gains a third row of pictures: Home's Watch Together row and the Dashboard
(owner, 20 November).

- **Dashboard:** shows Now Playing with two people watching (Sam on the Living Room TV and Kids on
  an iPad), the server graphs and recent activity.
- **Watch Together:** shows two invitations on Home. Sam is hosting Spider-Man: Into the
  Spider-Verse with Alex already in the room; Alex has invited Jordan to a Severance episode.
- **How they're made:** both come from the shared screenshot library. Its new `playing` and
  `watchTogether` entries say who watches what on which device. `web/tools/screenshots.mjs`
  signs those devices in with v1 device tokens, reports playback and makes the rooms just before
  the two pictures. This happens in README mode only, so `--serve` stays as it was.
- **Backdrops on Home (owner, 20 November):** the library now sets Home's look, with Continue
  Watching as Backdrops (each title's backdrop with its logo). The script saves it as the admin's
  default Home layout, which every app reads through the profile's preferences, so apps
  photographed against `--serve` show the same. `home.png` and `watch-together.png` were retaken.
- **`--only`:** `--only home.png,watch-together.png` saves just those pictures, so a library change
  that affects one page doesn't rewrite the rest.

Decisions:
- Home's Watch Together row rather than the lobby. The lobby is a small panel over an empty
  player, because the screenshot library's video is a blank generated clip.
- A host already in another room only invites, because the server allows one room per profile.

Checks:
- README mode run, and all seven pictures looked at; then `--only home.png,watch-together.png`
  run after the Backdrops change, and both looked at (the other five unchanged).
- No Go or web app code changed; `cd web && npx tsc -b` passes.
- No owner server, data, media or keys used.

Logging: none; this is a developer tool.

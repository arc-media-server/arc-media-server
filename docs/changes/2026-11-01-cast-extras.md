# Cast links for trailers and extras (1 November 2026)

## What changed

`POST /api/cast` now also takes an extra's stream link (`extras[].stream_url`,
`/api/extras/{id}/stream`), and the cast link it returns serves that extra. Until now it answered
400 "not a stream link", so the LG/Samsung app's extras player (which goes through `/cast`, as its
TV's player can't send the sign-in) couldn't play trailers, and neither could the new Roku app.

## Decisions

- The grant names the extra (`extra:<id>`), so the link opens that extra only, not its title's
  file or another extra. The stream itself still checks that the profile can see the title.
- Additive: no contract lines change.

## Logging

Nothing new: a failed lookup is logged once by the existing failure path; refusals are ordinary
client errors.

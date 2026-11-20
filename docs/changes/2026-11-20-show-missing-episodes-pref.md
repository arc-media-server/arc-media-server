# Show Missing Episodes as a profile setting

**20 November 2026**

## What changed

A profile preference, `show_missing_episodes` (off unless turned on), in `GET`/`PATCH /api/v1/me/prefs`
and the admin's `/profiles/{id}/prefs`. When it's on, an app lists a show's episodes that aren't on
disk among the ones that are, by asking for `GET /series/{id}/seasons?missing=1` as it already can.

## Why

The owner (20 November): Show Missing Episodes belongs in Arc's settings, not as a toggle beside a
show's season tabs. As a profile preference it follows the profile to every app, like Hide Spoilers.
arc-tv is the first app to use it (arc-tv PR linked from the PR).

## Decisions

- Off by default, as every app's toggle is today. Absent on older servers reads as off.
- Only the preference: the server doesn't apply it to `/seasons` by itself, so apps that don't know
  it keep working exactly as now, and an app can still ask for missing episodes when it wants them.
- The web's show page keeps its own Show/Hide Missing Episodes button for now; moving it, and the
  Android and Kodi apps, onto this setting are follow-ups.

## Checks

- `go test ./internal/api` (new `TestShowMissingEpisodesPref`: off by default, saved, kept by other
  changes, turned off) passed; `go vet ./...` passed.
- `go test ./...`: everything passed except `internal/transcode`, whose four failures need Arc's
  FFmpeg build (this machine has Ubuntu's FFmpeg 6.1, with no `tonemapx` filter or `dovi_rpu`);
  untouched here.
- `go run ./tools/apidoc`: four new contract lines, `.show_missing_episodes boolean` for each prefs
  endpoint; none removed.

## Logging

Nothing new: a preference save is an ordinary request.

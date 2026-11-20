# Watch Together rooms say whether they have started

Rooms gain `started` (boolean, always sent): false until someone presses Play, then true for the
room's life. Apps close the lobby when the room first plays. Until now they guessed this as
"not paused, or past position 0", which is wrong for a room made from a resume point: it starts
paused at a later position, so the host and everyone who joined skipped the lobby and landed on a
paused player with no Start button. The LG/Samsung app (arc-tv's Watch Together PR) found it in
two-client testing against a scratch server; Android, Apple TV and the web use the same guess.

Additive only: no field or route removed or renamed; `docs/api-v1-contract.txt` gains `.started`
for each room response. `docs/WATCH-TOGETHER-API.md` says when the lobby ends and the fallback
for older servers.

Decisions: a server flag rather than per-app guesses, because only the server knows whether a
paused room at 1:01 has never played or was paused there. Play (not the barrier releasing) sets
it, so the lobby closes for everyone as soon as the host presses Start, as in Plex's lobby.
Android, Apple and the web can adopt it in their own PRs (falling back to the old guess).

Checks: go vet ./... passed; go test ./...: 36 packages passed, internal/transcode failed four FFmpeg tests (DV fixture, remux offset, picture subtitle canvas, HDR tone map) that fail identically on unchanged main here because this machine's FFmpeg lacks the Jellyfin filters (e.g. tonemapx). The room manager and HTTP tests assert a room made at 1 s is
not started and is after Play), go run ./tools/apidoc. No web code changed. No owner server,
media, settings or keys used.

Logging: none added; this is a field on existing responses.

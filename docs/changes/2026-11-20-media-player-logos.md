# "Arc Media Player" logos for the apps

The owner asked for a second set of the white and black logos reading "MEDIA PLAYER" instead of
"MEDIA SERVER", since the apps are players rather than the server.

`docs/brand/` now has `arc-player-logo-white.svg`, `arc-player-logo-black.svg` and the stacked
`arc-player-logo-stacked-white.svg` / `-black.svg`, plus a preview sheet
(`arc-player-brand-sheet.png`); the README says the apps use them.

- **How they were made:** the same TV mark and "arc" shapes as the server logos; "MEDIA PLAYER" is
  Quicksand Bold drawn as shapes at the exact size, letter spacing and baseline of the existing
  "MEDIA SERVER" (fitted against it to within 0.01 units), so the two sets sit side by side
  identically. "PLAYER" is a little narrower than "SERVER", so the side-by-side file is 1.2 units
  narrower and the stacked one stays centred.
- **Decision:** the icon and the TV mark have no wording, so they are shared and unchanged. Plex
  calls its apps just "Plex"; the store name of each app stays as it is, this is only the logo
  artwork.
- **Not done:** switching the apps over (each app repository copies the files it uses into its own
  assets in its own PR).

Checks: rendered old and new side by side on black and white (`2026-11-20-media-player-logos/`).
Logging: none needed (artwork only).

# No UI colour: white and black instead of teal

## What changed

The owner dropped a UI accent colour across Arc's apps (1 November): controls that were teal
(primary buttons, chosen chips and tabs, switches, badges, focus rings, ticks, selection) are now
white with black text on the dark theme and near-black with white text on the light one. Arc's
teal stays only in the logo (the sidebar's mark and the app icon).

- `web/src/index.css`: the `--accent`, `--accent-strong` and `--accent-foreground` tokens are
  now white / near-black, so every component using them follows without other changes.
- Places that paired `bg-accent-strong` with a hard-coded `text-white` use
  `text-accent-foreground` (card actions, watch state, artwork chooser, filter count, Activity
  and News badges); the switch's thumb turns dark when on.
- The year-in-review card's teal gradient is a neutral grey one; the API docs page's accent is
  black/white.

## Decisions

Logged on #80 (owner's call). The Apple TV app changed first; Android, LG/Samsung and Roku follow
in their own PRs.

## Logging

No change: colours only.

# The Arc logo set

The owner chose the logo: the TV-on-little-feet mark (design 9 of the logo rounds) with the
original arc and dot inside, and the name in Quicksand Bold, lowercase "arc" over "MEDIA SERVER"
in spaced capitals, the text block exactly as tall as the TV (the top of "arc"
level with the screen's top, the base of MEDIA SERVER level with the feet). Chosen over "Arc" and
"ARC", Outfit, and one-line and name-above layouts.

`docs/brand/` holds the logo (white and black), a stacked version, the app icon and the mark,
as SVG with the lettering converted to shapes, and a README saying which to use where.

Where they're used (owner): the main logo in the web app's top corner (sidebar) and on the
sign-in/setup screens, drawn in currentColor so it is white in dark mode and black in light; the
TV mark alone on the phone bar, where search needs the room; the stacked logo at the top of the
README (white or black to match GitHub's theme, replacing the heading it spells out); and the app
icon as the web app's tab/home-screen icons (a full square, which browsers and phones round or
mask) and, uploaded by the owner, the GitHub organisation's picture (`arc-icon-1024.png`). The
logo's spoken label is translated (`common:productName`).

Checks: `npx tsc -b`, production build, i18n tests, the setup journey, and screenshots of the
sidebar (dark and light), phone bar and sign-in screen. Next: a bolder mark for favicon sizes,
and the TV apps.

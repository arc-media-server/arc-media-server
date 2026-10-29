# Poster hover controls: a proper select circle, and feedback on every control

From the owner's screenshot of a poster on hover: the select circle (top left) was a thin white
ring that didn't look right beside the unwatched count, and nothing showed the hover buttons were
clickable.

What changed (`web/src/components/CardActions.tsx`, `PosterCard.tsx`; posters and landscape
thumbnails alike):
- **The select circle is one control in both places.** On hover and while choosing titles it's the
  same 24px circle: a crisp 2px white ring on a dark, slightly blurred disc, so it reads on pale and
  dark artwork. It's the height of the unwatched count opposite and sits the same 6px in from the
  corner. Chosen, it fills with the accent and shows the tick (as it did while choosing titles).
- **The controls stay lined up with the artwork.** The poster lifts 4px on hover but the hover
  controls didn't, so the circle sat lower than the count badge. They now lift with it.
- **A soft shade over the artwork on hover**, darker at the top and bottom, as Plex does, so the
  white circle, pencil and ⋯ read on light posters. It sits under the watched badge.
- **Every control answers the pointer**: the circle fills with the accent, previews the tick and
  glows; Play scales up slightly, fills with the accent and glows; the pencil and ⋯ get a soft
  round highlight (⋯ keeps it while its menu is open). All show the pointer cursor, change within
  150ms, and have a white focus outline for keyboard users, with the same highlight as on hover.
- **While choosing titles**, hovering an unchosen card previews the tick in its circle, since
  clicking anywhere on the card chooses it.
- With reduced motion, Play doesn't scale and the controls don't move; they still fade and change
  colour.

Unchanged: what the controls do, their names for screen readers, keyboard behaviour, and touch
screens (no hover, so the plain card).

Checked with screenshots in dark and light themes, posters and landscape thumbnails, hovering the
circle, Play and pencil, keyboard focus and the chosen state.

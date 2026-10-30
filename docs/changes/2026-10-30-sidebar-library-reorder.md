# Reorder libraries in place in the sidebar

Follow-up to #293 after the owner's review of the sidebar.

## What changed

- **Quieter ⋯.** Each sidebar library keeps its ⋯ visible (no hide-on-hover), but at rest it uses
  the subtle text colour; it goes to muted on row hover or keyboard focus within the row, and to
  full strength when hovered, focused or open.
- **No grip icon.** The grip button beside "Libraries" and the unlabelled one in the phone top bar
  are gone, with the old `LibraryNavigationSettings` dialog and its locale keys.
- **Reorder libraries… in the ⋯.** Admins get it as the last item of the shared `LibraryMenu`,
  after a separator. Other profiles had no ⋯; they now get one holding just "Reorder libraries…"
  (`LibraryOrderMenu`). On phones it sits in the profile menu, after Settings.
- **Editing in place (desktop).** Choosing it turns the sidebar's library list into an editable
  list (drag handles with pointer and touch, Move up/down buttons, position announcements) with the
  ⋯ buttons hidden, and Cancel and Done under the list, plus a small "Reset to server order" text
  action. Done saves the profile's `library_order` through the same prefs API as before. A failed
  save keeps the draft and shows the error with Retry. Escape cancels. Focus starts on the first
  row's controls and returns to the ⋯ that opened editing.
- **Phones** open the same list in a dialog with Cancel and Done.

## Reuse

`ArrangeRow.tsx` gains `ReorderList`, the shared piece of Edit Home's reorder mechanics: Motion's
`Reorder.Group` of `ArrangeRow`s, the move buttons and the screen-reader position announcement.
Edit Home, Edit Recommended and the library order (sidebar and phone dialog) all use it now,
instead of each wiring its own group, move function and live region. `ArrangeRow` has a `dense`
size for the sidebar. The library order state lives in one hook, `useLibraryReorder`
(`LibraryReorder.tsx`), shared by the sidebar and the phone bar.

## Decisions

- With a single library there's nothing to reorder, so the item is left out; a non-admin with one
  library therefore has no ⋯.
- Saving an order identical to the server's stores no preference, so the profile keeps following
  the server order.
- "Reorder libraries…" is in the sidebar ⋯ only, not the ⋯ on a library's own page, since the
  editing happens in the sidebar.

## Access

Unchanged: only the libraries the server lists for the profile take part, and new libraries still
appear after the arranged ones (`navigationLibraries`). The journey checks that an inaccessible ID
in a profile's order never adds that library.

## Logging

Client-side only; no new server logging. A failed save is shown in place with the server's error
message; the prefs request already goes through the server's request logging.

## Checks

`npx tsc -b`, `npx tsc -p e2e/tsconfig.json` and `npm run build` pass. The rewritten journey in
`web/e2e/webserver.spec.ts` covers keyboard reorder and save, failed save with Retry, Cancel,
Escape, reset, a non-admin's ⋯, the phone profile-menu path and the absence of the grip button.

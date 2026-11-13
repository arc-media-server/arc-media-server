# Horizontal row navigation and Home hero visibility

Owner request, 26 October: desktop users need to reach cards beyond the visible part of a
horizontal row, and each profile should be able to hide the large Home banner.

## Horizontal rows

Poster/thumbnail rows, cast and Extras now share RowShell. Previous/Next buttons remain visible
in the row heading whenever content actually overflows; they are not dependent on hover, desktop
breakpoints or a fixed cast count. Each arrow disables at its end. When every card fits, controls
are omitted. Resize and content changes recalculate the bounds. Navigation moves approximately
one screen with overlap; existing card sizes, links, actions and native scroll snapping remain.

Each row has an accessible heading, scoped button labels and a focusable cards area. With that
area focused, Left/Right browses horizontally and Home/End reaches the ends. Tab still reaches
normal card links/buttons; focusing an offscreen card reveals it horizontally. Keys used inside
card controls and modified browser shortcuts are left alone. Reduced-motion devices avoid smooth
arrow scrolling. Native horizontal wheel/touch scrolling remains; ordinary vertical wheel input
continues to scroll the page. This does not implement TV remote or native-app navigation.

The shared shell is separate from MediaRow so cast/Extras do not import its playback/card graph.
Season tabs and other small tab bars retain their existing native navigation; this change targets
horizontal content-card rows.

## Home hero

**Settings → Your profile → Home → Show home hero** defaults On. Turning it Off hides the featured
banner for that profile and brings the rows up without a blank hero-sized spacer. Edit Home stays
reachable in a normal toolbar when there is no banner, preventing overlap with the new row arrows.

`PATCH /api/me/prefs` accepts `show_home_hero: true/false`, and the existing admin profile-preference
endpoint uses the same field. The value is saved with that profile's existing JSON preferences;
no schema migration, device-only storage or server-wide setting is introduced. Absent values
preserve the visible default; null patches follow the existing boolean preference convention
and leave the current choice alone. Wrong types are rejected. Unrelated preference patches and
admin-default row layouts preserve the choice; another profile retains its own default/value.

Saving uses existing optimistic updates, rollback and Retry. When hidden, Home does not initiate
the banner's featured-title queries, show its stale error or mount its rotation timer. Ordinary
Home rows and metadata/artwork remain unchanged. Re-enabling restores normal banner behavior.

## Reference and scope

[Plex's Web customization guide](https://support.plex.tv/articles/customizing-plex-web/) describes
customizable Home content and browser-associated layout preferences with optional settings sync.
Arc's hero switch is the owner's explicit request and uses its existing server-profile preference
model; this is not a claim that Plex has an identical toggle. It preserves the existing visible
banner by default and leaves row ordering/library settings separate. The choice is logged on #80.

## Verification boundaries

READINESS records actual local checks. Browser journeys use temporary stores, generated media,
local provider fixtures and pinned Chromium. Larger rows, seven-member cast lists, Extras and
hero artwork are injected UI fixtures to exercise layout; they do not establish provider matching
or real-library scale. Checks cover mouse/keyboard, both ends, focus reveal, resize, native wheel,
normal vertical scrolling, phone geometry, save failure/Retry, restart, banner query suppression
and default-on behavior. API checks prove per-profile isolation and partial-patch persistence.
Physical owner-PC input devices, other browser engines and production deployment are not claimed.

Current repository policy starts no Actions for PRs. Relevant checks, including the full browser
suite for this shared UI change, run locally before the PR. The owner merges; no direct main push.

## Poster-size controls and Home header (12 November 2026)

Home now uses the same page heading as Library: Home on the left; a poster-size slider and
three-dot actions menu on the right. Edit Home is inside that menu (the profile-menu link and
`?edit=1` remain). The duplicate bottom Edit Home button is removed. Library's slider sits next
to its existing right-hand actions menu on every tab; ordinary profiles get the slider without
receiving administrative actions.

The current profile independently saves `home_poster_size` and `library_poster_size` through
GET/PATCH `/api/v1/me/prefs` (the existing admin profile-pref routes expose the same fields).
Both are integer percentages, 75–150; missing/0 retains the current 100% default. Library size
is shared across that profile's libraries. Existing preference patches are atomic and preserve
unrelated fields; no database migration or extra service. Invalid sizes return400 without a
partial write. No fields or existing contract promises are removed.

Dragging previews immediately; pointer release, keyboard key release or blur saves the final
value, with one in-flight save per control. A failed save rolls back and offers Retry. A failed
initial preference read explains the disabled Library slider and offers Retry. Slider labels
are translated and native range controls retain keyboard/touch behavior.

Only poster cards and poster/collection grids scale; landscape/backdrop cards retain their
current sizes. Density does not affect selection, filtering, row identity, poster aspect ratio
or stored metadata. The slider is independent of the optional Recently Added caption PR #449.

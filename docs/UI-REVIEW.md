# Everyday UI and feature-gap review

Reviewed 22 October 2026 against `554120b`. This is an assessment, not approval to implement
every suggestion. No product behaviour or owner data was changed. The previous #144 bug pass
and #147 scanning work are completed context, not outstanding findings.
Tracking: [review #148](https://github.com/JonzaUK/arc-media-server/issues/148),
[R1 #149](https://github.com/JonzaUK/arc-media-server/issues/149),
[R2 #150](https://github.com/JonzaUK/arc-media-server/issues/150),
[R3 #151](https://github.com/JonzaUK/arc-media-server/issues/151),
[R4 #152](https://github.com/JonzaUK/arc-media-server/issues/152). All findings remain open.

## Assessment

Arc has substantial feature coverage. The next useful work is making existing capabilities
reliable, discoverable and consistent, rather than another large feature milestone. In
particular, users must distinguish empty libraries from failed requests, know whether a change
saved, and reach the same filtered results from Home and the library browser.

| Area | Already present | Gaps and recommended direction |
| --- | --- | --- |
| Finding common tasks | Sidebar, mobile navigation, global search, item menus, Library Health, Activity | Mobile management links form a long horizontal strip; Settings is a long unindexed page. Group profile/server settings and make sections directly reachable. |
| Home and discovery | Per-profile rows, hide/reorder, custom filters, seasonal picks, collection/playlist rows, default/admin layouts | Failed requests resemble empty content. Some custom rows stop at 24 with no full-results route. Keyboard reordering is absent. Keep the existing editor and improve these paths. |
| Library management and browsing | Direct background add, optional preview, global scanning, paging, genre/language/label/decade/quality/unwatched filters, several sorts, bulk actions | Home/smart-rule filters additionally support added date, runtime and minimum rating; normal browsing cannot express those. Unify the filter contract and add a full-results route. Sort direction and remembered browse preferences are later usability options. |
| Movie/series details and ordering | Edits and locks, artwork, versions/editions, provider orders, comparison, custom placements, missing/unplaced episodes | Failed order saves are silent. Scope is insufficiently explained: admin changes also become the server default. The reset-to-inherited-default API has no ordering-menu action. Explain source numbering versus display order without adding new concepts to the ordinary Play flow. |
| Playback, audio, subtitles, queue, resume | Real direct/remux/conversion journeys, resume, next episode, shuffle/playlists, audio/subtitles/quality/chapters, PiP | Phone controls become narrow. Audio preference is learned by choosing a track but lacks an explicit profile-settings field. There is no visible editable now-playing queue or Play Next/Add to Queue action. Error screen offers Close, without an explicit Retry action. |
| Profiles, permissions, settings | Household/friend profiles, PINs, access rules, admin-only APIs, individual preferences and admin home defaults | Profile saves can fail without visible feedback. Phone profile rows/header need restructuring. Clarify the separation between profile preferences and server settings. No new permissions bypass was established by this pass. |
| Desktop and smaller screens | Responsive sidebar/top navigation, wrapping forms, card grids, compact dialogs | At 390px Profiles exceeds the viewport; a normal household name becomes barely readable. Player controls fit the tested state but shrink to about 23px wide. Improve layout/touch affordances; this is not proof of actual phone/browser support. |
| Loading, empty, error, recovery | Shared skeleton/empty/error components; several forms already show errors; playback has conversion fallback | Home conflates errors with emptiness, some mutations hide errors, profile-preference load can remain a skeleton, and the shared ErrorState has no explicit Retry. Standardise these states and preserve unsaved input on retry. |

## Confirmed findings

### R1: Home reports an empty server after a failed library request — P2

With an existing generated movie library, return HTTP 503 for `GET /api/libraries`. After both
query attempts, Home displays “Welcome to Arc” and Add a Library instead of the error. A failed
Continue Watching request also removes that row without an error. Legitimately empty rows may
remain hidden; failed rows need a distinguishable state and retry.

Evidence: focused browser check `review: failed home dependencies resemble empty content`.
Source: `web/src/pages/Home.tsx:101`, `:147`, `:148`; `MediaRow`/`RowShell` hide empty arrays.
Do not claim that the underlying library or resume data was deleted.

### R2: Failed profile preference and episode-order saves are silent — P2

Return 503 for a Hide spoilers preference PATCH: the optimistic switch rolls back, without
showing the response error. Return 503 for an episode display-order PUT: the menu closes and
keeps Aired Order, without explaining why. Surface a local error and a retry route. Extend the
same review to Refresh, ratings and Continue Watching removal; those are source candidates,
not separately reproduced failures in this pass.

Evidence: `review: preference and episode-order failures have no feedback`.
Source: `web/src/pages/Settings.tsx:36`, `web/src/lib/home.ts:12`,
`web/src/pages/Series.tsx:70`. The admin server-settings load error is already handled; do not
confuse it with the profile-preferences loading/error gap.

### R3: Profiles layout breaks down at phone width — P2

At 390×844, with Admin and a household profile named Audit Household, the document is 403px
wide. The header actions exceed the viewport and the household name collapses to a tiny label
between the avatar and Home/Edit/Remove controls. Wrap or regroup actions and give profile
identity adequate width. Test household and friend rows with long names.

Evidence: actual browser geometry and inspected `profiles-390.png`.
Source: `web/src/components/Page.tsx:26`, `web/src/pages/Profiles.tsx:63`, `:106`.

### R4: Home row reordering has no keyboard route — P2 accessibility

Focus a row's Drag button and try Space/ArrowDown/Space: the row order is unchanged. There are
no Move up/down controls. The drag handle starts only on pointer down. Provide keyboard move
actions and useful focus/announcement behaviour; apply the same review to playlist ordering.
Playlist keyboard behaviour was inspected in source, not reproduced separately here.

Evidence: `review: custom row has no full-results route and reordering requires pointer`.
Source: `web/src/components/EditHome.tsx:356`–357.

## Confirmed feature gaps and design proposals

1. **One filter model and full results from Home.** A custom row using runtime, added-date or
   rating filters has no heading link. A synthetic response with 60 matches shows only the
   requested first 24, with no View all or paging. This is a navigation/feature gap, not evidence
   that files are missing. Extend normal library browsing to the same filters, then link rows to
   their complete matching list. Source: `Home.tsx:117`, `lib/home.ts:180`, `Library.tsx:45`.
2. **Make display-order scope explicit.** The API writes the admin's selection both to their
   profile and to the server default; ordinary profiles override only themselves. The menu says
   files are untouched but does not explain that scope or expose the existing null/reset API.
   Recommend scope-aware help and Use server default for individual overrides. Preserve the
   approved provider/episode identity and read-only-media rules. Source: `internal/api/series.go:91`
   and `web/src/pages/Series.tsx:378`.
3. **Organise Settings.** The seeded admin Settings page at 390×844 measured roughly 8,473px
   tall with 20 h2 labels, including Server and repeated Subtitles labels. This varies with data
   and platform. Group Profile, Library/metadata, Playback/transcoding, Access/devices and
   Maintenance/recovery, with section navigation. Keep advanced controls inside their relevant
   group; do not turn ordinary configuration into more switches.
4. **Improve mobile playback controls.** Real generated video advanced in the 390px Chromium
   viewport. The measured Play/Subtitles/Quality/Speed buttons were about 23.3×40px. They remained
   on screen; clipping was not established. Prefer a small primary transport row plus a menu
   for secondary controls. Real touch, orientation, safe-area and mobile browser testing remain
   part of #135.
5. **Expose an explicit preferred audio language.** Selection already persists through the
   player; add an understandable profile-settings field and explain whether a playback change
   changes the lasting preference. This is UI discoverability work, not a missing audio engine.
6. **Consider queue controls after the above.** Arc can play playlist/shuffle queues and advance
   episodes, but PlayerContext exposes only starting/replacing playback. A queue viewer, remove/
   reorder, Play Next and Add to Queue would be useful additional features. Treat them as a
   separate product change; keep display ordering and resume semantics intact.
7. **Recovery affordances.** Add explicit retries to query errors and appropriate playback
   failures; preserve dialogs/input and explain permission/provider/storage failures. First
   address the confirmed silent errors, then review remaining mutations and loading states.

## Recommended sequence

1. Fix R1/R2: honest load/save states, then meaningful regression tests asserting the corrected
   behaviour. Extend review to the source candidates without assuming every mutation is broken.
2. Fix R3/R4 and mobile player layout. Verify desktop, 390px, keyboard and long-name cases.
3. Unify filters/full results and explain display-order inheritance; organise Settings and add
   explicit audio preference. These are proposed UX changes, not implemented work.
4. Decide queue enhancement scope separately. Keep native apps (#63), downloads (#62),
   translations (#122), specials placement (#109), API docs (#139), and real-library/device/
   sustained-use work in their existing issues. Music, Trakt and plugins are not added here.

## Evidence and limits

The repository's existing 23 regression journeys had passed in the preceding implementation
turn. This review added four separate diagnostic journeys, run against the real embedded app,
temporary databases, generated media and local provider fixtures. All four reproduced the
observations above; that means the diagnostic checks passed, not that these bugs were fixed.
The synthetic 60-match response isolates row navigation and is not a 60-file scanner test.
Review checks and their own TypeScript check pass; see `web/review/README.md` for reproduction.
Screenshots/logs stay ignored under `web/test-results/ui-review/`.

Source inspection covered all eight requested areas; browser reproduction concentrated on the
findings above. No claim is made that every dialog, mutation, keyboard path, provider, device
or responsive size was exhaustively exercised. No owner database/settings/media was opened,
and no Unraid trial or new hardware/casting acceptance was performed.

## Plex references

Plex documents [home/recommendation management](https://support.plex.tv/articles/manage-recommendations/),
[preferred audio/subtitle settings](https://support.plex.tv/articles/204985278-account-audio-subtitle-language-settings/)
and [Play Next/queue actions](https://support.plex.tv/articles/202188298-play-queues/). These inform
the comparison and proposals above; Plex documentation is not proof of Arc behaviour or a
complete client-by-client feature-parity audit. Keep Arc's existing simpler flows where they work.

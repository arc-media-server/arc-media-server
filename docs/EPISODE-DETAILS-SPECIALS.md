# Episode details and inline specials

The metadata-scope portion of this original contract is superseded by
[METADATA-ORDERING.md](METADATA-ORDERING.md), 26 October 2026: library provider selects show
information; episode information/credits follow file numbering, and per-show Details From
is removed. The traversal, identity, spoiler and provider-availability boundaries remain.


Implemented for #106/#109 after the owner's 25 October review. These controls enrich
episode browsing without changing media files, canonical episode identity or watch history.

## Cast, crew and appearance counts

The episode row's **Details** button opens a keyboard-accessible dialog independently of
Play, including episodes missing from disk when Show Missing is enabled. It shows the stored
description, director/writer credits and guest cast. Loading failures offer Retry. Existing
Hide spoilers hides both description and guest appearances until watched; no request is
started for a spoiler-hidden dialog. Series cast cards show a supplied positive aggregate
appearance count, across the provider's whole show, not just episodes in this library.

Credits follow the show's **Details From** provider. TMDB season guest_stars/crew are saved
against the canonical episode's stable Arc ID, using the existing provider episode mapping when
numbering comes from TheTVDB. Episode titles, descriptions, images, runtime and source numbering
retain their existing canonical-provider policy. This is not general cross-provider backfill.
Missing mappings or missing provider credits remain unknown. TheTVDB's current episode response
has no supported TMDB-person credits: choosing it clears previous TMDB credits/counts rather
than silently retaining another provider. Returning to TMDB restores the supplied data.

TMDB total_episode_count is used directly; multiple character totals are not added together.
Unknown/zero totals are omitted, not displayed as zero. Existing bounds retain the first 20
actors/guest stars and director/writer roles. Shared person identity, multiple role de-duplication
and existing field locks remain. No per-episode network request happens when browsing or opening
the dialog; GET /api/items/{id} reads the local store and now returns episode credits too.

## Specials in seasons

The show's **Episode Order** menu offers **Show specials in seasons**, initially Off. Like
display-order choices, a normal profile changes its own preference; an admin also sets the
server default inherited by profiles without an override. The independent reset action restores
inheritance (or Off for the admin default). Changing episode order does not erase this preference.
Save failures retain the previous setting and provide Retry. Custom/manual arrangements win.

TheTVDB aired metadata's positive airsBeforeSeason and optional airsBeforeEpisode are resolved
to stable Arc episode identities. An absent/zero episode hint means before the first known episode
of that season; a missing, negative or unmapped target produces no guessed placement. Multiple
specials for a target follow their original Special numbers. Targets can be missing from disk;
their known metadata position still defines where a special belongs. Unknown targets/specials
remain in Specials, or the existing Not in This Order bucket when the chosen order omits them.

An unlocked S00 episode with a known target is displayed just before that target in its selected
season. It retains source season 0, its number and Arc's **Special N** label. It is not duplicated
in the Specials tab. A custom order or manually placed special is never rearranged. An alternate
provider order uses the same mapped target identity, not that provider's possibly different
season/episode numbers. Source files and source-ordering rows are never rewritten by the toggle.

Store.DisplayEntries supplies the same traversal to season browsing, Next Up, player Next/autoplay,
series-to-queue/playlist expansion and season-wide watched actions. Non-inline specials retain
their existing separate playback lane. Repeated positions for a multi-part episode remain intact
for playback's last-position rule; the UI still shows each identity once. Toggling affects future
queue construction and Next requests, not already saved playlists or explicit queues.

PATCH /api/series/{id}/specials accepts inline_specials true, false or null. It uses the same
profile visibility checks as series browsing and does not grant access or start a scan.
The orderings response includes the effective inline_specials and specials_overridden flags.

## Existing libraries and upgrade

Migration 0052 preserves older people/credit rows with unknown counts, creates stable placement
and scoped preference tables, and leaves specials Off. It does not start a bulk rescan, network
refresh, file analysis or history rewrite. Existing shows receive new credits/placement hints
through **Refresh metadata** or the already configured metadata maintenance policy. TheTVDB
must be configured and its aired data mapped for its placement hints to be available. A partial
metadata import without aired data retains earlier hints; a complete aired refresh replaces stale
hints. Refresh retains edited titles, files, watch data and manual/custom entries.

Rollback across the migration requires a compatible pre-upgrade database/full-data backup as
usual. There are no new dependencies or background services. AMS_TVDB_URL mirrors the existing
test-provider URL override so browser verification can use local fixtures without real keys.

Actual test counts, screenshots, failed attempts and hosted publication are recorded in
READINESS.md, #106/#109/#162 and /workspace/arc-cloud-evidence/episode-details-specials/.
Generated media/provider fixtures do not establish real-library mapping completeness, provider
data accuracy, device behavior, owner deployment or public-release acceptance.

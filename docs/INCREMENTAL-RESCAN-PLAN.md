# Routine scans and metadata maintenance

Owner approved, 23 October 2026. Tracking: #169. Baseline ab9ff78.

1. Ordinary scans check files and local sidecars, preserve successful inspection and title
   identity, discover/match new or pending content and recover unfinished initial imports.
   Remove age-only movie/show refresh and old unmatched retry from this path.
2. A genuinely new, unlinked episode may request current provider metadata when cached
   numbering cannot place it. Existing unknown files must
   not cause another provider request simply because another scan ran. Explicit source-order,
   provider/language and manual refresh actions retain their identity/history-preserving behavior.
3. Metadata maintenance drains incomplete/stale movie pages, handles due/airing shows and old unmatched
   movie/show retries. Honor the existing enabled switch, maintenance admission and foreground
   cancellation; Run now works outside the clock window. It never discovers filesystem content.
   Maintenance provider failures must not schedule a discovery scan as a side effect.
4. Existing identical link writes already no-op. Batch current episode file links per group,
   compare fresh desired mappings and call the writer only for differences. Keep parsing and
   mapping validation so numbering/provider/manual changes cannot leave a stale identity cache.
   Report Checking episode links versus Linking episodes truthfully in Activity.
5. Avoid the writer queue for unchanged local artwork/theme/season posters. Keep source-side
   checks so added/changed/removed sidecars still take effect; preserve chosen artwork/locks.
6. Verify stale-but-unchanged scans make no age-only provider/probe calls; maintenance and
   explicit refresh still work; new episodes, source-order changes, local artwork changes,
   interrupted import retries, missing roots and watch history remain correct. Include proof
   that unchanged work can complete while the SQLite writer is held.
7. Use generated/temp libraries and local providers only. Run affected Go/race/API tests,
   vet/build and focused browser scan/Activity/Settings journeys. Preserve actual evidence and
   update handoffs. Real NAS timings/device/sustained-use acceptance remain separate.

Plex reference: ordinary scanning finds file changes and gets metadata as needed; explicit
refresh and scheduled maintenance handle existing metadata. Its internal database algorithm is
not public and no performance equivalence is claimed.
https://support.plex.tv/articles/200289306-scanning-vs-refreshing-a-library/
https://support.plex.tv/articles/201553286-scheduled-tasks/

Implemented and verified. Initial tests found a reused preview response on new-episode lookup;
that path now requests fresh data. See READINESS and #169 for exact checks and limits. Full
rescans still visit files/sidecars and validate associations; no folder-skipping cache is added.

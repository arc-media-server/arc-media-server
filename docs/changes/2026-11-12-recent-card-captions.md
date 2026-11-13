# Clearer Recently Added season and episode captions

Owner follow-up to the 24-hour grouping change (#446): grouped season cards show the show
name and only the season label (or Specials), leaving out the new-episode count. Series cards
retain their existing count. Single episode cards in Recently Added show three distinct lines:
show name, episode title, then zero-padded display-order code such as `S01 · E01`. Specials use
`S00`; missing display placement says Unplaced episode instead of inventing a number.

The shared card override accepts an optional third line, supported by poster and landscape
cards. Other rows keep their existing captions. Long titles truncate independently so the
number stays on its own line. Existing artwork, selection, watch badges, hover actions and
navigation remain; no API fields, stored arrival times or 24-hour grouping rules change.
New labels use the English Home catalog; obsolete season-plus-count phrases are removed.

Diagnostics: presentation-only, so no new logs, background work, per-card records or expensive
disabled-debug preparation. Existing request/error handling remains responsible.

Validation: Go vet/full tests (37 tested packages), production/app/e2e TypeScript and build
passed; all 13 affected browser journeys passed (49.5s, zero failures/skips/retries), and
desktop/phone captures were inspected. The first run had 12 passes and one incorrect fixture
menu-label expectation; the assertion was corrected and the final full affected run passed.
No race suite was repeated for this UI-only change. Actual logs/artifacts are retained under
`/workspace/arc-cloud-evidence/recent-card-labels/`. All checks use temporary stores,
generated media and local providers; no owner media/settings/keys or production access.

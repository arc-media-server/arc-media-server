# Singular plural forms (#122, 28 October)

Follow-up to the translation integration (#256 review): ten English `_one` forms were copies of
their `_other` ("{{count}} days ago", "{{count}} versions"). English output doesn't change
(English never shows those at 1, or now reads correctly when it does: "1 day ago"), but
translators start from `_one`, and in some languages "one" also covers 21, 31 and so on.

Fixed: common `activity.filesFound`, `settings.libraryIssues`, `time.daysAgo`; filters
`added.lastDays`; libraryManagement `health.duplicateShows`/`duplicateMovies`; reports
`year.binge`, `resources.cores`; titles `movie.versionCount` (and titleActions
`bulkEdit.mergeCount`, fixed while merging #253).

A new check in `tools/i18n.test.mjs` (run by every build) fails when a `_one` is word-for-word its
`_other` and counts a plural noun; it flagged exactly these nine before the fix.

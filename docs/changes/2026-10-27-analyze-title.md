# Analyze a single title (#213, 27 October 2026)

Like Plex's Analyze, the More (⋯) menu on movie and show pages (admins only) has **Analyze**.
It rereads that movie's files, or every episode's files for a show, with the same safe
reanalysis as Manage libraries → Reanalyze files: known present files only, no folder walk or
missing-file sweep, watch history/markers/edits kept, changed or offline files skipped.

It runs in the background under the library's lease (`StartItemAnalysis`, operation
`analyze_item`), so it never overlaps a scan or other library work. Activity shows
"Analyzing <title>" with progress and Stop, then "N analysed · N failed". The API is
`POST /api/items/{id}/analyze` (202, or 409 when the library is busy or no files are present);
it also accepts an episode ID.

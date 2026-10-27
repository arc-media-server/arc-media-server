# Manage libraries actions and Media tools (27 October)

Owner feedback: each library row mixed a text Scan button, a small ⋯ icon, a text Edit button and
a bin icon. Rows now show Edit followed by one More (⋯) menu, as Plex's library ⋯ menu:
Scan library files, Refresh all metadata…, Reanalyze files…, Empty trash…, then Remove library….
Stop still appears on the row while work runs; a pending scan shows "Requesting a scan…" in the
row's status. Scan all libraries and Add Library stay in the page header. No API changes.

Settings › Playback & transcoding › Media tools only appears when it has something to do:
ffprobe or FFmpeg wasn't found, or a custom ffprobe path was set (so it can be changed back).
When both are found automatically, the section and its link are hidden. Diagnostics downloads
still include the FFmpeg details.

Checks: tsc app/e2e, production build, and Playwright maintenance, ux, journeys and clarity
specs (78 journeys, including a new one for the row and menu order and Remove from the menu).

# Separate movie editions

Movies with named cuts have separate entries in the library. Multiple files for one cut,
such as 1080p and 4K, still share that entry. Web movie cards show the release year and
edition, separated by the same middle dot used for episode numbering. The movie page
links to other playable cuts the current profile can access.

Existing movie libraries adopt the split during a scan. The original item ID, profile
history, resume position and watchlist stay on the standard cut when present, or on the
edition of the oldest catalogued primary file. Other cuts get their own watch state.
Existing manual merge and split assignments, and ambiguous multipart groups, are kept together.
Name every part of a new multipart cut with the same edition. Files stay in place.

Validation used a disposable catalogue. Scanner and storage tests cover stable rescans,
matched metadata, and independent resume records. Chromium checked both edition cards,
navigation to the other edition, track reset on switching files, and successful playback.
No deployed server or native app was used. The additive API contract exposes edition names
for native apps that choose to display them. Scan errors use the existing scan diagnostics.

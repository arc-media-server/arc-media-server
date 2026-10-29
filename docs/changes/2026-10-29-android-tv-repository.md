# The Android TV app has its own repository

The first Android TV slice was built in this repository's `clients/android-tv/` (PR #298). The
owner decided it belongs in its own repository: it's a different language and toolchain with its
own release cycle, and the two people working on Arc know about API changes anyway. It moved to
**arc-media-server/arc-android-tv** with its full history (`git subtree split`), plus its own
licence (AGPL-3.0, as here), agent notes, PR template and change note.

This repository keeps: the v1 contract (`docs/api-v1-contract.txt`), which protects the app; the
ROADMAP status line under milestone 18; and server-side fixes for the API gaps the app found
(listed in that repository's change note and on PR #298). #298 is closed in favour of the move.

Later client licensing (e.g. a more permissive licence for an Apple app, because of the App
Store's terms) is a separate decision for the owner, noted on #80.

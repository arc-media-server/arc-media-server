# Timeline thumbnails as a BIF file, for Roku

**What changed.** `GET /api/v1/items/{id}/trickplay/{file}/bif` serves a file's timeline
thumbnails as one BIF file, the format Roku's own player shows while seeking. The trickplay
description (`GET /items/{id}/trickplay?file=`) gains a `bif` link beside `sheets` (additive).

**How.** Nothing new is made in the background. The first request cuts the thumbnails out of
the existing sheets (10×10, 320 px wide, one per 10 s) and writes `thumbs.bif` beside them in
`data/trickplay/<file>/`; later requests serve that file. Remaking or forgetting the thumbnails
deletes the folder, so a stale BIF can't outlive them. One BIF is cut at a time.

BIF layout: the 8-byte magic, version 0, the thumbnail count and the interval (10,000 ms) in a
64-byte header; then an index of (time in intervals, offset) pairs ended by `0xffffffff` and the
end offset; then each thumbnail as a JPEG.

**Why.** It's groundwork for a Roku app (owner, 1 November; see the "Arc on Roku" write-up).
Roku's player draws seek previews only from BIF. The endpoint is general `/api/v1`, not
Roku-only; any player that reads BIF can use it.

**Smaller artwork, considered and dropped.** Arc already serves card-sized pictures from TMDB
(w500 posters, w1280 backdrops), and Roku's Poster component scales a picture down as it loads it
(`loadWidth`/`loadHeight`), so a server-side resize isn't needed.

**Limits.** Checked with unit tests (byte layout, each thumbnail cut from the right place, the
file kept and removed with its sheets). It has not been shown on a real Roku; that waits for a
Roku app and a device. A two-hour film's BIF is about 720 thumbnails, a few MB, cut in about a
second the first time.

**Logging.** A failed cut is logged once as a warning with the file id. A 404 before the
thumbnails exist is ordinary, and it moves the file up the queue as the sheets endpoint does.

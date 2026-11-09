# #350: isolate unreadable audio during season intro detection

Real FFmpeg regressions reproduced a single video-only or corrupt episode aborting comparison
for all healthy episodes. Use optional audio mapping, skip files without usable fingerprints,
and compare the remaining episode neighbours. Parent cancellation still aborts, preserving
paused work; one fingerprint has a two-minute wall bound. Do not invent markers for skipped files.

The managed season job completes for an unchanged revision after isolated input failures instead
of being marked failed and re-fingerprinting all healthy files hourly. A changed file/season
revision is eligible again through the existing job bookkeeping. Unexpected file failures produce
one season summary warning with counts, duration, a filename and first safe error; ordinary
missing/too-short fingerprints are counted in the normal outcome. Never warn once per file.

Capture at most 4 KiB stderr and include a bounded first error line, redacting source path and
URLs. Cancellation remains quiet; no progress-loop logging or credentials/SQL/body logging added.

Generated common-intro/credits audio with an inserted video-only or corrupt episode failed first,
then retained markers for all healthy episodes. Tests also cover callback counts, bounded private
diagnostics and cancellation. Checked with the Docker image's pinned Jellyfin FFmpeg 8.1.3-2
package (8.1.3-Jellyfin binary), no owner media or physical-player acceptance. Failed inputs need
repair/reanalysis or a changed revision before automatic retry; no new retry UI is added here.

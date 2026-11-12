# Controlled conversion resource assessment (#193)

Record current-main measurements and distinguish existing ahead throttling, explicit Optimize,
optional maintenance and pending #372 storage protection. Two loaded and two control rounds use
Jellyfin FFmpeg 8.1.3 with generated 1080p media, temporary RAM-backed stores and two CPU cores.
See CONVERSION-RESOURCE-ASSESSMENT.md for methods, results, failed initial harness assumption,
limits, repeatable commands and next acceptance work.

All 1,296 browse responses and 91 scripted media responses succeeded. Concurrent Optimize kept
both cores busy and live-plan readiness increased from 1.5–1.7s to 2.7–3.3s; this is not a physical
player or NAS benchmark. Live capacity, delete/cleanup and real permission-failure retry checks
worked. Eleven main safeguard tests and two separately pending #372 tests passed with no skips.

No product, scheduling, API, schema or logging behavior changes. Keep #193 open for actual disk/
GPU/owner-server acceptance and a deliberate policy decision. No new runtime logs/volume are
needed for a documentation-only measurement report. No owner data/media/keys or persistent server.

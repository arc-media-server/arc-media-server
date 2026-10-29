# Intel runtime drivers and explainable hardware detection

The amd64 image explicitly installs Debian's free intel-media-va-driver and i965-va-driver.
The existing --no-install-recommends FFmpeg install did not ensure VA driver availability.
No non-free repository or host-driver installation is added. Other image architectures retain
existing packages; actual encoder support still depends on the GPU, host kernel/firmware and
container device permissions. Intel Coffee Lake/UHD630 is a supported driver generation, not
proof that every host/device configuration works.

Detection enumerates up to four render nodes, tries the selected node for QSV/VAAPI, and can try
iHD/i965 and low-power VAAPI encoding. It retains capped failure reasons and the successful
node/driver/mode; playback uses that tested configuration. The test uses generated blank H.264
frames, never owner files. Attempts have eight-second bounds within a 45-second total job budget;
process output is capped while collected. Bounded results appear in admin Settings and diagnostics.

Settings now says hardware encoding is unavailable rather than declaring the GPU unsupported.
Admin Retest starts a coalesced asynchronous detection job, retaining prior working capabilities
until the result replaces them. Detailed failures use debug logging; normal logs add only detection
start/outcome summaries, with no polling logs. Existing redaction applies to retained errors.

Validation: transcode/API tests, focused detection race tests, vet, UI build/app+e2e TypeScript,
20 translation checks, embedded build and two pinned browser journeys passed. Fixtures cover a
non-default render node, driver/low-power fallback, immutable diagnostics, bounded output,
coalesced retest, UI failure/Retry and non-admin denial. A disposable Debian bookworm runtime
image built the exact package step, verified both driver libraries and encoded generated video
with libx264. This is runtime packaging validation, not the full multi-stage release image or
physical UHD630 validation. No host tools were installed or owner settings/media accessed.

Driver/package references: https://github.com/intel/media-driver and
https://packages.debian.org/bookworm/intel-media-va-driver . Debian retains package copyright/
licence notices. Free driver builds and host firmware can have capability limits; failed actual
trials remain unavailable and provide their error rather than enabling a listed encoder blindly.

API-reference follow-up: regenerated both docs/API.md and docs/openapi.json after the UI build;
`go test ./internal/apidoc` passed. The local combined-PR check caught the stale machine-readable
reference before handoff. No product behaviour changed in this follow-up.

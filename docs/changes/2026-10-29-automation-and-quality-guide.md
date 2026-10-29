# Automation and remote quality guidance

Documented the existing versioned artwork/collection API, administrator session authentication,
image formats and limits, membership constraints, retry/idempotency limits and the need for an
Arc adapter for Plex-oriented tools. Added a README link. No new API or credentials were added.

Clarified existing remote quality help: compatible files within both bitrate and resolution
limits retain original quality; other playback constraints still apply. Upload speed must be
measured on the server's connection. Added REMOTE-QUALITY.md explaining the current 10% tolerance,
unknown-bitrate handling and why a browser speed test is not server upload measurement.
Automatic WAN measurement is deferred pending an explicit external service choice.

hdrprobe (https://github.com/matthane/hdrprobe, MIT) is a possible later source of richer Dolby
Vision/mastering/MaxCLL information. It is not installed or bundled here. Existing stored
ffprobe/HDR inspection fields should be presented first; richer parsing needs bounded sampling,
licence/dependency review and real-format fixtures. Native output validation remains separate.

Validation: documentation cross-checked with auth, artwork, collection and playback handlers;
production translation/app TypeScript build. No owner-server or third-party adapter test.
No new logging: documentation/help-only change, existing playback decisions remain logged.

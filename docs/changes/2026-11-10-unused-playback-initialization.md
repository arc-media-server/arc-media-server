# Remove redundant playback audio initialization

The review's playback.go SA4006 item is still present on main: the initial
defaultAudio result is overwritten on both branches before use. Declare the
audio pointer without doing that unused lookup. The later version-description
lookup remains active and is retained. This is a behavior-preserving code-quality
cleanup, not a measured playback-speed improvement.

Part of #350, grouped with #397's private unused-code cleanup. No new logging,
API schema/contract, frontend, media or dependency changes.

Validation: go vet ./..., go test ./... and API race passed (250.935s).
Other #397 touched-package race results remain the earlier recorded checks;
their source did not change in this follow-up.

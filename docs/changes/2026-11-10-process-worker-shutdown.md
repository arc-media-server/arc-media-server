# Drain process workers before closing the store

Only scanner work was joined previously, and ordinary request contexts had no
process parent. Early startup returns could close the store before canceling
other background writers. The review established this broad finding by reading;
new deterministic ownership tests and Linux process smoke checks validate the fix.

A named process worker owner now starts the recurring loops and dynamic callbacks,
stops admission, cancels the root and joins tracked work. HTTP requests are tracked
until they return. Only then does shutdown join asynchronous API work, avoiding
WaitGroup Add/Wait races, and finish the scanner, webhook/artwork, conversion and
subtitle drains before Store.Close. Startup-error returns use the same drain.
Hardware rechecks use the process root rather than a detached request context.
Pending replacement listeners are closed and producers cannot block on shutdown.

Ordinary requests inherit the process root. A separate listener-closing signal
still ends SSE/room streams during port changes, without canceling accepted
background jobs. Failed graceful HTTP shutdown force-closes active connections;
fatal Serve errors also take the cleanup path. A slow worker/drain gets one
bounded warning with static operation labels and counts after five seconds in
Close. It keeps waiting, preserving dependencies until the work actually exits.

Library previews now use the process root and async wait group. Baseline bulk
refresh/import launches are owned too. Preserve #377's richer bounded queue and
#398's atomic profile mapping/session lifetime/outcome logging during integration.

The exact prerequisite production changes from #409 (conversion Close), #410
(subtitle job ownership) and #411 (webhook/artwork Close) are included so this
branch is independently buildable and exercised. Retain each once when merging.
Also preserve #355 HTTP read/idle limits, #376 settings lock changes, #400 session
cleanup logging, and other main.go/api.go security and API fixes.

Tests cover held workers/requests, post-stop admission, late async registration,
safe bounded diagnostics, root cancellation and listener-rebind semantics.
Two disposable-database Linux process smoke checks passed: SIGTERM exited 0 and
occupied-port startup failure exited 1; both databases passed quick_check. No
child processes happened to be active in those smoke samples; actual FFmpeg
reaping is established by #409/#410's dedicated generated-media/process tests.
No physical Docker-stop, NAS, Windows, GPU or device acceptance is claimed.
Library-managed ACME renewal is not redesigned; this change owns Arc catalog
workers and FFmpeg lifetimes. No owner data/media, dependencies, persistent server
or frontend changes. Part of #350 API Low 9.

API generation adds only the preview's shutdown-unavailable response; no contract
fields, success responses or types are removed.

Validation: go run ./tools/apidoc, go vet ./..., go test ./... and serial
touched-package race tests passed: lifecycle 1.048s, API 248.204s, artwork 1.020s,
notify 3.250s, subtitles 1.239s, transcode 7.704s; cmd/ams has no unit tests.
The initial non-escalated loopback test was blocked by sandbox socket permissions;
its proper-access rerun passed. Smoke results/logs are retained outside the repo.

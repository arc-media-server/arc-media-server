# #350: browser MIME, framing and content protections

Real handler regressions confirmed the production app, API docs, API and error routes had no
nosniff/framing headers. Add X-Content-Type-Options: nosniff and X-Frame-Options: DENY at the
outer request boundary, including early authentication/cross-origin errors. Production web
assets, SPA routes and the bundled API docs also receive a Content-Security-Policy.

The policy blocks inline scripts, dynamic evaluation, plugins and framing; scripts load from
Arc and the existing opt-in Google Cast SDK origin. Inline styles remain for React/Scalar.
Provider images/media, blob HLS workers and HTTP/WebSocket connections retain their supported
schemes. This deliberately broad connection/media allowance preserves existing playback and
provider features; it is not a complete DOM-XSS audit or a claim that external URLs are safe.
CSP is not added to API/media responses. No HSTS: Arc still intentionally supports plain local
HTTP. Missing-build fallback remains plain text with the outer MIME/framing protections.

Pinned-browser production tests verify first-run UI, API docs, blocked inline script execution
and a blocked cross-origin iframe. The broad browser suite is run because the policy affects
all web pages. Physical Chromecast, televisions and deployment-specific reverse proxies need
owner/device validation; the suite's casting receiver is a fixture. No owner media or persistent
server is used. No new logs/report endpoint, per-item work or secrets; protections are fixed
response headers and add no log volume. No API field/type changes.

Actual validation: Go vet/full tests and API/webui race passed; production build (including app
TypeScript) and both new focused browser tests passed. The full pinned-browser run finished
155 passed / 10 failed, not a green suite. Seven failures were reproduced on unchanged main
with an isolated alternate-port harness: ambiguous library menus, hover play interception,
old Edit Home/settings text and a changed series menu expectation. The other three (two
30-second optimization waits and queue-resume ordering) passed on main control and then all
passed on a focused header-branch follow-up after shared CPU load subsided. Original logs and
failure artifacts are preserved. Existing e2e TS6133 remains the explicit final #350 item.
No broad-suite or physical-casting acceptance is claimed from those follow-ups.

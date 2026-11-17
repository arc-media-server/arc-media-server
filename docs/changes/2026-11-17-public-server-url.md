# Public server URL for reverse proxies

Plan: persist a validated optional HTTP(S) origin, use it for remote status/copy/check and
remote cast/webhook links, keep LAN addresses and router/TLS settings independent, and verify
round-trip/clear/validation/proxy behavior plus the settings UI before opening a PR.

The public_url setting is blank by default. Saving https://arc.example.com makes it the
remote address without appending Arc's internal port. Clearing restores inference. Paths,
credentials, query strings and fragments are rejected; subpath hosting is not introduced.
Saving performs no DNS/network request. Router port mapping remains independently controlled
by UPnP: turn it off for a reverse proxy. The proxy terminates public TLS; Arc's own TLS remains
optional. Neither discovery infrastructure nor DNS/proxy/router provisioning is added.

Custom-address checks use normal TLS verification, a six-second limit, bounded health bodies
and no redirects. Existing direct-address self-signed certificate checks keep their behavior.
A successful check proves an Arc health response, not that the external server identity was
cryptographically verified or that another network can reach it. Failures do not echo URLs.

Remote cast grants and enabled-remote webhook links use the configured origin. Direct LAN
casts keep their request/LAN address. Trusted-proxy HTTPS headers remain trusted only from
configured peers; public_url does not grant local sign-in bypass or trust forwarding headers.
No persistent connection registry or automatic app address switching is added.

Logging: existing settings mutation/request failures and explicit check results remain the
boundary. No new URL, per-request info, credential or per-item logging. A custom address is
configuration, not a background job. Tests use temporary stores and local stand-ins only.

Reference: Plex's official Network documentation describes Custom server access URLs for reverse
proxies (https://support.plex.tv/articles/200430283-network/). Arc intentionally supports one
explicit origin here and has no plex.tv-style registry; it does not publish a remote discovery
listing or automatically switch existing app connections.

Validation (Go 1.27.0, prepared Jellyfin FFmpeg 8.1.3, pinned Chromium rev1248):
- Full `go test ./...`: 37 packages passed, 7 without runnable tests. `go vet ./...`, production
  i18n/unit/TypeScript/Vite build, e2e TypeScript and final embedded-server build passed.
- Affected remote/settings/API/entrypoint race checks passed (1.360/7.296/70.848/7.383s).
  Additional final API race checks (10.877s) cover trusted versus untrusted proxy schemes and
  prove the public URL does not grant password-free home sign-in. Contract/doc checks passed;
  generated API/OpenAPI/v1 changes add only the public_url settings field.
- 2/2 pinned Chromium journeys passed at 1280px and 390px (17s): save/reload/normalize,
  configure the proxy URL and turn UPnP off before enabling access, reject path-prefixed URLs,
  clear to automatic detection, preserve router settings, and show correct proxy HTTPS guidance.
  Screenshots inspected; no horizontal overflow or uncaught page errors. No full browser suite.
- Initial sandbox socket checks were blocked and rerun with fixture permissions. The first
  browser attempt used synchronous check() on an asynchronously saved switch; corrected to
  click and await the persisted state, retaining assertions. Earlier logs retained as evidence.
  Working-directory-only command attempts were corrected before final builds/doc generation.
- No owner server/settings/media/production keys, Unraid, tool installs, physical-device or
  real reverse-proxy acceptance. No persistent server. Whitespace checks passed.

Configuration/client handoff: docs/PUBLIC-SERVER-URL.md. Saving a URL never changes UPnP;
its controls are available before enabling remote access once a custom URL is saved. Copy/check
use the origin without Arc's internal port. Existing check results reset when the address changes.

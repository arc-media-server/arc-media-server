# Trusted proxies (#80 item 41, 28 October)

Settings → Access & devices → Network has a **Trusted proxies** field (empty by default), as
Plex's and Jellyfin's known proxies. Add the address of a reverse proxy such as SWAG or Nginx
Proxy Manager (one per line or comma-separated; single addresses or ranges).

- Requests from a listed proxy use the client address it reports (Forwarded `for=`, else
  X-Forwarded-For, else X-Real-IP). The chain is read from the nearest hop back, skipping other
  trusted proxies, so a visitor can't pretend to be at home by sending their own header first.
  "unknown"/obfuscated clients are treated as not home.
- That address decides home sign-in (Skip the password at home), remote bandwidth limits, the
  HTTPS redirect for people away from home, sign-in rate limits, sign-in logs and the Dashboard's
  address. Before, everything behind a proxy shared the proxy's rate limit.
- A trusted proxy's `X-Forwarded-Proto: https` / `proto=https` counts as HTTPS, so Arc's own
  HTTPS redirect doesn't loop behind a TLS-terminating proxy.
- Unlisted senders' forwarding headers still grant nothing: such requests are never "home",
  exactly as before. With the field empty nothing changes.

Checks: go vet/test (new auth tests for home/remote clients, spoofed chains, trusted hops,
RFC 7239 with IPv6, X-Real-IP, obfuscated clients, untrusted headers and HTTPS reports), tsc
app/e2e, production build with the i18n audit, and Playwright: a new journey against the real
server (127.0.0.1 standing in for the proxy) plus Settings/ux, recovery, webserver and i18n specs.

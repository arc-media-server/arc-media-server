# Public server URL and reverse proxies

In Settings → Access & devices → Remote access, set **Public server URL** to an address such as
`https://arc.example.com`. Include the scheme; an optional explicit port is supported. Use an
ASCII/punycode hostname or an IP literal. Blank restores Arc's public-IP/outside-port detection.
The setting can be entered before remote access is enabled and survives server restarts.

For a reverse proxy:
1. Configure the proxy and DNS to route that public hostname to Arc's internal listener.
2. Turn off **Open the port on the router** (UPnP). Arc's outside-port field controls only direct router
   mapping, independently of the port in your public URL. Saving a URL does not alter UPnP.
3. Let the proxy manage public HTTPS. Arc's own HTTPS can stay off when the upstream connection
   is HTTP; deployments using HTTPS upstream can retain Arc's certificate settings.
4. Configure **Trusted proxies** for the proxy peer and have it send correct forwarding headers.
   This is independent of public_url; the URL does not trust headers or bypass sign-in.
5. Enable remote access and use **Check**. Copy and the displayed outside address use your URL.
   Also test from outside your home network; a check from inside isn't proof of outside reachability.

No custom path prefix is supported: use a dedicated hostname, not `https://example.com/arc`.
Credentials, query parameters and fragments are rejected. No DNS or connection check happens on
save. A check requests only `/api/health`, with no cookies/credentials, no redirects, normal
certificate verification for custom HTTPS origins, a six-second request timeout and a bounded
response. It verifies an Arc health response, not server identity or external reachability.

## API and app handoff

- `GET /api/v1/settings` (admin) adds string `public_url`, default `""`.
- `PATCH /api/v1/settings` (admin) accepts `{"public_url":"https://arc.example.com"}`;
  the returned settings contain the normalized value. `{"public_url":""}` clears it.
  `null`, invalid schemes, non-root paths, credentials, query strings and fragments return 400.
- `GET /api/v1/remote` (admin) retains its shape. When remote access is enabled and public_url
  is nonempty, `url` is the configured origin. The service skips external-IP lookup; `public_ip`
  and `double_nat` are not synthesized for that mode. Router/certificate status remain independent.
- `POST /api/v1/remote/check` retains `{reachable,error?}`; custom TLS failures, redirects and
  non-200/non-Arc health responses fail. Errors do not echo the configured URL.
- Remote cast grants and webhook links use this origin only while remote access is enabled.
  Direct LAN casts keep the browser/LAN address. Existing stream authorization, session
  revocation, profile access and trusted-proxy rules remain unchanged.

This does not register the server in an internet directory, add a public connection-discovery
endpoint, alter local `/discover`, or rewrite addresses already saved in native apps. Existing
apps can connect directly to the domain through the proxy; no app update is needed for this
settings feature. Client developers should not treat admin settings as a public discovery API.

Reference: Plex's [Custom server access URLs](https://support.plex.tv/articles/200430283-network/)
serves a similar configuration use case, but publishes multiple URLs to plex.tv. Arc stores one
explicit origin and has no central registry.

# Named tool tokens and additive label filters

Tools can now obtain their own ordinary device token from Settings → Access & devices →
Devices → **New token for a tool**. The administrator names it and copies the credential once.
It then appears in the existing Devices list marked **Tool**, with creation/last-used information
and a confirmed Revoke action. Non-admin profiles cannot create or manage tokens.

## API and storage

- `POST /api/v1/devices/tools {"name":"Library automation"}` returns HTTP 201 with `token` once;
  the response is `Cache-Control: no-store`. Send `Authorization: Bearer <token>`.
- These are ordinary device sessions: the creating admin starts selected, existing session
  authorization and hash-only storage are reused, and device revocation immediately denies
  subsequent requests with 401. In-flight work may finish. Password changes and stale-device
  cleanup apply exactly as for other devices; this is not a separate credential/expiry system.
- Migration 0071 adds only `sessions.is_tool`, a display marker (existing devices default false).
  `GET /api/v1/devices` adds `is_tool`; no existing field or endpoint is removed. The normal
  automatic before-upgrade backup applies. Last-used bookkeeping remains best-effort and bounded,
  normally at five-minute intervals, without synchronous writes on every authenticated request.
- Existing admin-only `POST /api/v1/items/bulk` already adds/removes labels and genres atomically.
  The PR reuses and tests it. Unrelated labels remain; labels never lock; removal wins if a name
  is in both lists. No refresh, provider call or scan is required for a label edit.
- `GET /api/v1/items?label=Weekend&label=Favourite` now matches **any** supplied label, ignoring
  case, while retaining other filters and profile exclusions. Existing label facets remain
  access-controlled and are covered by the regression. Excluded titles cannot be recovered by
  requesting their label explicitly.
- The generated API reference, OpenAPI and additive v1 contract are updated. The client guide has
  a "Building tools for Arc" walkthrough: obtain a token, resolve TMDB/IMDb/TVDB IDs, label titles
  and create/populate collections.

## Decisions

[Plex's official API](https://developer.plex.tv/pms/) uses `X-Plex-Token` account authentication
and also permits those headers as query arguments. Plex tools use the account's token. Arc
issues a separate named device token per tool so one can be revoked alone, uses its existing
Bearer header, and never generates credential-bearing URLs. There is no Plex/Jellyfin API layer:
Kometa or another integration would need an Arc API adapter.

ANY is the repeatable-label default: it makes combined label selections a useful union, with
all other filters still narrowing the list. An ALL mode would add an option without a demonstrated
need. Tools receive the admin's existing rights, rather than introducing scopes or another
credential system. The tools marker has no effect on authorization.

## Logging

One info outcome on successful tool-token creation, without the name, credential, hash or URL.
Failures and revocation use existing API diagnostics. No per-item label logs or new request-loop
logging. Privacy tests exercise detailed logging, including a rejected query-string credential.
The plaintext token stays only in the creation response and component state; it is not retained
in the browser's query/mutation cache or persistent storage.

## Validation

Actual final checks and limitations are recorded in the PR. The change includes regressions for
ordinary-session storage/revocation/cleanup, hash-only persistence, admin admission, no credential
in logs, bulk label preservation, repeatable-label ANY filtering and access-controlled facets.
The Settings journey covers create/copy/one-time display, the tool marker, cancelled and confirmed
revocation, non-admin hiding and narrow-screen overflow. The existing Settings journey is retained.

[Screenshot](2026-11-21-tool-tokens-labels/tool-tokens.jpg) uses synthetic tool metadata and contains
no credential. No production data, provider keys, owner server or media was accessed. No third-party
tool integration, physical client or sustained deployment acceptance is claimed.

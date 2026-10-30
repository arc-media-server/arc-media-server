# Server identity, capabilities and contract corrections

Backend handoff for Claude. UI work is intentionally separate. Apps use `/api/v1`; examples omit
that prefix. Authenticate with the existing session Bearer token and select a profile first.

## Server identity

`GET /identity` (profile) returns:

```json
{"id":"<32 hexadecimal characters>","name":"Arc Media Server","version":"<build>","api_version":1}
```

The random instance ID is generated once in SQLite migration 0060. It survives renames, normal
updates/restarts and backups taken after identity was introduced. Restoring such a backup restores
that ID. A pre-identity backup generates an ID when upgraded. A cloned database preserves its ID:
this is not a multi-server cloning/discovery solution. Treat IDs as opaque identifiers, never as
credentials or proof of trust. Scope app caches to server identity and profile. Do not discard
locally saved credentials merely because a server is temporarily unreachable.

`GET /settings` (admin) includes `server_name`. `PATCH /settings` accepts
`{"server_name":"Living Room"}`. Leading/trailing whitespace is trimmed; empty, null, non-string,
control-character and over-80-character names return 400 without applying the patch. Only admin
may rename. The public `/health` retains its existing product name and contract.

Claude: add the friendly-name field to the existing server settings UI and use the identity name
where appropriate. No new setup screen is required. A server name is separate from its URL and ID.

## Runtime capabilities

`GET /capabilities` (profile) returns `api_version` and a map of feature keys:

```json
{"api_version":1,"features":{"browse":{"supported":true,"available":true,"allowed":true},"metadata_edit":{"supported":true,"available":true,"allowed":false}}}
```

Missing key means this server does not advertise support. `supported` identifies implemented API
support; `available` reports attached runtime services; `allowed` reports the current profile's role
permission. These are separate axes. Item/library/age/label permissions are always enforced by the
operation itself. Availability can change; handle normal endpoint errors even after a positive hint.
It is not a claim that a provider, network, decoder, GPU, HDR display or receiver will work.

Keys: identity, browse, playback_options, playback, transcoding, playlists, collections,
artwork_palette, subtitle_delivery, trickplay, activity, maintenance, metadata_edit, artwork_upload,
library_scan. No paths, keys, tokens or private network addresses appear in this response.

Claude: use flags to tailor controls while retaining normal error handling. Refresh on profile
changes and server reconnect. Do not poll this aggressively; requests produce no success log lines.

## Contract fixes

- Settings PATCH now appears in generated API/OpenAPI and the additive v1 contract. Route discovery
  unwraps known decorators and fails on unknown wrappers instead of silently dropping the route.
- Request schemas are separate from output schemas. Pointer inputs support omission; explicitly
  annotated null resets and conditional/defaulted inputs are represented independently of output
  encoding tags. Optional password/PIN fields remain subject to runtime authentication rules.
- The old per-show `details_provider` input is deprecated and still rejected. Select the library
  provider, per-show file episode numbering and display order through their supported operations.
- Existing success-response fields/types remain protected. No client migration is required.

Errors use the existing JSON error envelope: 401 without a selected authenticated profile, 403 for
admin-only operations from an ordinary profile, 400 for invalid names, and 500 for storage failure.
New identity/capability requests add no info/debug logging or provider/media work. Rename uses the
existing settings outcome log; no credentials are included in these new fields.

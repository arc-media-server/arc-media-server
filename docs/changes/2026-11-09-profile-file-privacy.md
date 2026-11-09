# #350 / #80: full host paths belong to the administrator

Confirmed Unix/Windows-style source paths are returned to household/friend profiles through
single and batch item details. Probe-error text and library root arrays also expose host layout.
Follow the owner's #80 default: admins retain full source details; all other profiles get only
the filename. Keep the existing path field/type and all file ids/stream metadata.

Mask response copies at the shared item-details boundary (and matching response boundary), not
stored paths. Non-admin library root arrays are empty; folder navigation still uses indexed
relative API paths on the server. Probe and scan error detail is generic for ordinary profiles;
admins retain the actionable diagnostics. Artwork/extras' internal paths were already excluded,
and Health/Activity/settings filesystem diagnostics are admin-only. No changes to media, scanning,
streaming lookup, access checks or per-profile visibility.

Regression failed first for household/friend item and batch paths, raw probe text and library
roots. Fixed cases cover both path separator styles, admin-before/after ordinary reads, unchanged
stored file identity/path, generic scan-error detail/counts and existing Other videos navigation.
No real owner media, production system or physical deployment acceptance claimed.

No new logs or per-item processing beyond response string/copy masking. Existing request failure
boundaries remain; full paths are intentionally not printed to ordinary clients. No keys/tokens,
bodies, raw URLs or SQL arguments added to logs. API contract fields remain intact; clients must
use provided stream URLs and file ids, not interpret path as a local filesystem capability.

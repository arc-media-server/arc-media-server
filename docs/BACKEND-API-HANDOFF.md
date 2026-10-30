# Backend API handoff for Claude

This batch provides backend/API support only; reuse the existing shared web/app controls when adding
UI. Keep `/api/v1` integration additive and detect capabilities on reconnect/profile change.

Merge dependencies in order:

1. **#309 — API foundations:** [API-FOUNDATIONS.md](API-FOUNDATIONS.md). Fixes contract discovery and
   optional/default/null request descriptions; adds stable authenticated server identity, admin-friendly
   server name and permission-aware runtime capabilities. Claude adds the name field to the existing
   settings screen and uses identity/capability hints in clients.
2. **#310 — Show Unmatch:** [UNMATCH-API.md](UNMATCH-API.md). Claude adds an action to the shared show
   menu with a clear confirmation that metadata, overrides and display orders are reset. Explicit Fix
   Match re-enables metadata; ordinary scans do not. Files and saved user references are preserved.
3. **Server play queues (this branch):** [PLAY-QUEUES-API.md](PLAY-QUEUES-API.md). Claude connects the
   existing queue/player controls to profile/device-owned, persistent, revision-checked server state.
   Starting/stopping actual playback remains a client action using existing playback endpoints.

For stacked PRs, merge the predecessor, then retarget the successor to main before merging. Do not
merge descendants without their schema dependencies. No backend PR in this batch was auto-merged.

The first two guides live in their respective predecessor PRs; this final branch includes all three.
The generated docs/API.md, docs/openapi.json and docs/api-v1-contract.txt describe the resulting API.
Tests use temporary stores/local provider fixtures, not the owner server or media. No persistent Arc
server or physical-device/native rendering acceptance is claimed.

The wider API review still contains separate work such as event streaming, provider-ID/bulk lookup,
additional filters, recommendation continuation, richer playback negotiation, subtitle upload and image
variants. Those are not advertised as available by these PRs. PostgreSQL remains planning issue #308.

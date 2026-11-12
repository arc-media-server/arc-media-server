# Durable offline download bundles (#62, D3)

Persist safe prepared item/series/technical snapshots with the original checksum, and atomically
publish bounded copies of available local/uploaded/cached artwork and external SRT/WebVTT captions.
No provider call, FFmpeg extraction, original-media writes or full original duplicate. Optional
unsupported/missing assets are explicit. Add original-download capability only with the complete
supported bundle/transfer contract; converted/offline-sync capabilities remain absent.

Reader leases and catalog-safe periodic reconciliation protect active copies and reclaim orphans
following release/FK cascades/crash/restore. DB-only restored references cannot invent missing files.
Migration0070 invalidates old no-snapshot preparations for explicit release/reprepare. All API
changes are additive, with generated reference and native-client handoff in DOWNLOADS-OFFLINE-API.

A metadata-drift regression failed against D2 after correcting an initial test type; it passes with
persisted snapshots. Real copy/digest, HTTP1/2 caption/range, private-data, reader/cleanup, cascade,
reopen/missing-copy, publication-failure and storage-budget checks accompany original transfer tests.
An accidental test-file overwrite was caught at compile and corrected before final verification.
Actual vet/full Go/affected races and remaining physical-device/deployment limits are in the PR.

Existing bounded preparation and transfer outcomes remain responsible; cleanup failures warn at
most once per minute. No source/provider path/URL, auth value, private key, SQL args or chunk logs.
No frontend/native code, progress sync, converted downloads, production data or merge to main.

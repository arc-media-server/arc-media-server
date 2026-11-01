# Docs-only merges don't rebuild the image

## What changed

- The Docker image workflow no longer runs for a push to main that only changes Markdown files
  (change notes, README, ROADMAP, docs) or `docs/api-v1-contract.txt`. The image would be
  byte-for-byte the same.
- Everything else still builds as before:
  - Version tags.
  - Manual "Run workflow" with full verification.
  - Any merge that touches code, the web app or `docs/openapi.json` (the server embeds it).

## Why

To save Actions minutes, the owner asked (1 November) that only the commits that need a build start one, in
every repo. The other workflows are already manual only, or the repos have none:
- arc-tv, arc-android and arc-roku have no workflows.
- arc-apple's macOS build runs on request.

## Limits

- GitHub checks paths per push. A merge mixing docs and code still builds, as it should.

# CLAUDE.md slimmed down (28 October)

CLAUDE.md had grown to 812 lines, about 640 of them a running diary ("Where things stand") that
every Claude session loaded. The diary moved unchanged to `docs/HISTORY.md`; CLAUDE.md is now
about 210 lines.

Its replacement is a short status (alpha, milestone 21, open owner items, where history lives,
licence) and a **Workflow** section that collects the standing rules previously buried in the
diary: PRs only with local checks listed, owner merges, change notes in `docs/changes/`, decisions
logged on #80, configuration in the web UI with Plex-like defaults, no completion claims without
real evidence, and the settled decisions (no Jellyfin-compatible layer, no media deletion, one
metadata provider per show without backfilling). Principles, layout, build/test, TV orderings and
gotchas are unchanged. AGENTS.md points to the new Workflow section and `docs/changes/`.

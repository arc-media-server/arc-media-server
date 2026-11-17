# Slimmer agent notes: one shared rulebook

**What changed.** The rules every Arc repository shares (workflow, PRs, change notes, decisions,
never-do rules, Arc's design, the API promise, logging basics, working with the owner) now live
once, in `docs/AGENT-RULES.md`. This repository's `CLAUDE.md` imports it and adds only the
server's own status and rules (about 620 words, down from about 2,740). The reference material it
used to carry (setting up a new machine, code layout, build and test details, how TV orderings
work, gotchas) moved unchanged to `docs/DEVELOPING.md`, read when needed. `AGENTS.md` (Codex)
points at both.

Each app repository's notes now keep only their own rules plus the must-never rules, and point here
for the rest (matching PRs in arc-android, arc-apple, arc-kodi, arc-roku and arc-tv).

**Why.** The owner asked (17 November): every agent notes file is loaded into every turn of a
session, so long files cost on every message. All six repositories' notes came to about 5,700
words when cloned together.

**Decisions.** The must-never rules (no pushing to main, never the owner's data or keys, no
unevidenced device claims, Co-Authored-By only) are repeated in every repository, because an app
session doesn't load the server's files on its own. No rule was dropped; each moved to the
rulebook, a repository's own notes, or `docs/DEVELOPING.md`.

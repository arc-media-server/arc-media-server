# A logging rule for every feature

The owner asked whether PRs should add logging, without slowing Arc down.

- **CLAUDE.md, Workflow, "Logging"** (applies to Claude and Codex):
  - Info: one line per outcome of background work, and decisions worth explaining later.
  - Warn/Error: every failure, saying what failed.
  - Debug: per-item and per-request detail, recorded only while Settings' detailed logging is on.
  - Never at Info or above inside per-file, per-request or per-segment loops, and never keys,
    passwords or tokens.
  - Each PR says what it logs.
- **PR template** (`.github/pull_request_template.md`): What changed / Logging / Checks run.
- **Gaps filled in today's features:**
  - A show refresh (also the background part of Fix Match) logs "show refreshed" with how long it
    took, or "refreshing show failed" with the error. Stopping it isn't logged as a failure.
  - The Recommended tab logs its row count and time at Debug only, because it runs on every visit.

Why it doesn't slow Arc: a Debug line is skipped before anything is formatted while detailed
logging is off. The cost of logging comes from Info lines in hot loops, which the rule forbids.

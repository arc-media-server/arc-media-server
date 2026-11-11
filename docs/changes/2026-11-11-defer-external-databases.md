# Defer external database support

Owner decision, 11 November 2026: focus on getting Arc stable and ready for release with
SQLite. PostgreSQL and other external databases can come later; maintaining parallel query,
schema-upgrade and backup paths is outside the current release focus.

Issue #308 stays open as deferred work. PRs #337/#339 retain their code and historical
verification unmerged. Do not resume that project or require ongoing PostgreSQL compatibility
for new SQLite features unless the owner asks. Revalidate compatibility with current main,
schema history, backups and recovery before resuming. Downloads planning can target Arc's
existing SQLite migration/backup system.

CLAUDE.md, ROADMAP.md and the backend handoff record this decision for both agents. No runtime,
database, dependency or API changes; no existing implementation/evidence is deleted. This note
changes priorities, not release acceptance or the separate review requirement for draft #418.

Validation: reviewed documentation diff and local link targets; git diff --check. No test-suite
rerun for this prose-only follow-up. Existing PR #419's code/schema checks remain recorded at
their tested commit. No new logging is required for a planning/documentation-only decision.

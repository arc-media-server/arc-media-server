# Focus agent context on the current task

Shorten the server AGENTS.md entry point by removing duplicated workflow/status text and the
obsolete milestone-3 warning. Server status remains in CLAUDE.md; shared safety and workflow
requirements remain in docs/AGENT-RULES.md.

Add shared guidance for all Arc agents to load reference documents only when relevant, read
unchanged instructions once while available in context, search for relevant sections, keep tool
output bounded and preserve concrete progress in handoffs. Required instructions, prerequisites,
cloud startup and validation checks still apply. Existing app entry points already reference
these shared rules, so app repository changes are unnecessary.

Decisions: keep short critical entry rules and centralise context guidance in the existing shared
rules. No product or Plex behaviour changes.

Validation: reviewed the documentation diff and local reference targets; git diff --check passed.
Runtime, compiler and browser checks were not run: only Markdown instructions changed.

Logging: no runtime behaviour or logging changes.

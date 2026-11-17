# Decisions go in PRs, not #80

**What changed.** #80 ("Owner decisions") is closed and kept as history. Agents now note the
product defaults a change picks in the PR description, in a new "Decisions" section of the PR
template. When the owner really has to choose, the agent opens one issue per decision in this
repository, labelled `owner decision`, with a recommendation.

**Why.** The owner asked for it (17 November). #80 had grown to about a hundred comments mixing
settled choices, defaults and open questions, so open questions were hard to find and to close.

**Open decisions moved out of #80:**
- #477: Apple app licence and App Store distribution.
- #478: TheTVDB access for public releases.
- #479: one name for subtitle timing across apps.
- #480: "More Like This" or "Related" on title pages.

Older docs that say a choice was "recorded on #80" still point at the right place: #80 stays
readable. The agent notes in every app repository (CLAUDE.md or AGENTS.md) say the same as this
repository's.

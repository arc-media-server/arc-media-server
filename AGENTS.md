# Arc Media Server: instructions for Codex and other agents

Read `CLAUDE.md` in this repository before starting work. It is the shared source of project
instructions, architecture, principles, build/test commands, decisions and current status.
Read `ROADMAP.md` and `docs/READINESS.md` for the active plan and required verification evidence.

The project is now in milestone 21, stability and release readiness. Earlier milestone-3 status
from an older checkout is obsolete. Preserve the owner's database, settings and media; use
isolated test data for checks. Never mark real-device or sustained-use validation complete from
simulations. Track completed work in GitHub, record each change in `docs/changes/`, and keep the status in `CLAUDE.md` current.

For Codex Cloud development, also read `docs/CODEX-CLOUD.md` for environment setup,
isolated verification, cross-device handoff and the branch/PR workflow. Cloud task
state does not replace GitHub commits or establish actual deployment validation.

Workflow rule (owner, 26 October): all additions and fixes use pull requests, for Codex and
Claude alike. Never push to main; use a branch (`codex/...`) and open a PR. PRs start no GitHub
Actions runs: run the relevant checks yourself first and list the results in the PR. One agent
per branch. See the Workflow section in `CLAUDE.md`.

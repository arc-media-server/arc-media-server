# Arc development in Codex Cloud

The owner approved using Codex Cloud for cross-PC development on 22 October 2026,
with GitHub remaining the source of truth. This file prepares the repository; it does
not establish that an account environment is created, published or verified.
Setup and outstanding account checks: [GitHub #157](https://github.com/JonzaUK/arc-media-server/issues/157).

## Create the environment

In ChatGPT/Codex choose **Work in > Cloud > Select environment > Create environment**,
or **Settings > Codex Cloud > Environments**. Select only
`JonzaUK/arc-media-server` and keep the environment private (**Only me**).
Sign in and authorize the repository connection yourself if prompted.

Give the environment setup conversation this request:

> Prepare this private repository for Arc development. Read AGENTS.md, CLAUDE.md,
> ROADMAP.md, docs/READINESS.md and docs/CODEX-CLOUD.md. Install the Go version in
> go.mod (currently 1.27.0), Node 24, npm, FFmpeg/ffprobe and a C compiler for Go's
> race detector. Run `bash tools/codex-cloud/setup.sh`, then
> `bash tools/codex-cloud/verify.sh`. Fix environment setup failures without changing
> product behaviour or weakening checks. Use only temporary stores, generated clips
> and local fixture providers. Record actual results and any unsupported checks.
> No real provider keys, owner database/settings/media, Unraid access or persistent
> server is needed. Report the configuration and verification before publication.

The setup command installs locked npm/Go dependencies and Chromium plus its system
dependencies, then builds the embedded UI. Language runtimes, FFmpeg and a compiler
must already be installed by the environment preparation. The verification command
runs Go vet/tests/race, a disposable server build and all browser journeys. It does
not start the ordinary `go run ./cmd/ams` server or open the owner's `data` folder.

The **Codex Cloud preflight** GitHub workflow runs these exact commands on a fresh
Ubuntu runner whenever the scripts change, or when manually dispatched. Its result
validates this Linux recipe; it does not create or verify a Codex account environment.
The existing Docker release gates remain independent.

Allow package installation during setup. Required sources include npm, the Go
module proxy/checksum database and toolchain downloads, Linux package mirrors and
Playwright's browser download hosts. Let setup report missing destinations rather
than allowing unrestricted networking. GitHub access is needed to fetch/push the
selected repository. No TMDB/TVDB or production secrets are needed for these tests.

Review actual setup results, save, and **Publish**. Publication is an account action;
a Git push alone does not create or publish a Codex environment. Start a fresh task
from the published environment and confirm the same checks work there. Record its
environment name, starting commit and results in the handoff or GitHub issue.

Do not create a broad reusable service that automatically runs every time this
environment starts: the test harness manages its own isolated servers on ports
18199/18200, temp databases, generated videos and cleanup. Docker installation and
actual GPU/TV/Unraid acceptance stay with GitHub CI or real deployment checks.

## Continue from either PC

Reopen the **same cloud task** to continue its conversation and saved work. A new
task starts separate work; it does not inherit every prior chat. Task state is not
a replacement for source control. Keep completed work and a concise handoff in Git.

Start a task with:

> Continue Arc's milestone 21 stability and readiness work. Read the repository
> instructions and current notes first; fetch GitHub and inspect the task checkout
> before updating it. Do not discard uncommitted work. The owner prioritizes
> Plex-like simplicity and fixes to actual Arc behaviour. Scanning controls, the
> everyday UI/queue review, series-folder serialization, queued SQLite writers,
> retained diagnostics and poster-only saves are implemented; inspect newer commits
> before assuming they are still pending. Real-library/device/sustained-use evidence
> remains open on #134–#138. Confirm the present baseline and outstanding issue evidence
> before choosing more work. Use isolated test data. Commit and push completed work,
> update shared notes and GitHub, and report checks with their limitations.

Cloud development uses a `codex/` branch and a pull request for review and CI. Attach
created PRs to the task. Merge to main only when authorized; main publishes Docker
images after its gates. Do not run a local and a cloud agent on the same branch at
the same time. Local PCs can pull completed changes with `git pull --ff-only` when
their checkout is clean; never force/reset/stash unknown local work automatically.

## Saved context and present baseline

- `AGENTS.md` points agents to shared instructions; `CLAUDE.md` records architecture
  and current state; ROADMAP/READINESS and GitHub hold plan and verification evidence.
- Before cloud preparation, local and GitHub main matched `a0de3eb`. All release
  gates and image publication passed in Actions run 39816525377. Local production UI,
  Go vet and Go tests also passed after syncing nine laptop commits.
- Current automated browser coverage has 38 journeys. Run it; do not treat this
  historical count or prior results as fresh cloud verification.
- Account sign-in, repository authorization, environment publication and a successful
  fresh cloud-task run must each be verified separately. Preparation on Windows or
  passing GitHub CI is not proof that the cloud environment works.

Official setup and saved-state behaviour:
[Codex Cloud environments](https://learn.chatgpt.com/docs/environments/cloud-environments).

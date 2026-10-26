# Development images and full verification

Owner-approved policy, 26 October 2026: short feedback loops for the owner's private server.
`latest` is a development image. Normal pushes must not wait for the complete browser/race/
container suite before the owner can try a fix.

| Trigger | Work | Published tags |
| --- | --- | --- |
| Push to main | Build both architectures, then publish immediately | latest and full commit SHA |
| Manual run, Full verification checked (default) | Linux Go/vet, race, full browser suite, embedded build, container lifecycle/upgrade/rollback, image build | Commit SHA, only when all required checks pass |
| Manual run, Full verification unchecked | Build and publish | Commit SHA |
| Push a v-prefixed version tag | Full verification before publication | Version tag and commit SHA |
| Pull request | Nothing runs (owner, 26 October): agents run checks locally, the owner tests | None |

Manual/version runs do not move the development latest tag, so a slower verification run cannot
overwrite a newer main testing image. Failed/cancelled builds never publish. Verification runs
require both verification jobs to succeed; missing/skipped/failed gates cannot publish a release.
New main pushes cancel superseded main image runs. Manual/release verification has its own
concurrency group and is not cancelled by a normal development push.

The Dockerfile still compiles production TypeScript and the Go server. The same image health
check remains. Main does not run or claim passes for Go tests, race, browser journeys or container
acceptance; those workflow jobs are deliberately skipped. Focused local checks appropriate to a
change still provide feedback during development. The full tools/codex-cloud/verify.sh remains
available unchanged. The Codex Cloud preflight is manual-only.

Pull requests start no GitHub runs (owner decision, 26 October, after Actions minutes ran out).
Agents run the checks relevant to a change in their own environments before opening a PR (Go
vet/tests, TypeScript, production build, affected browser journeys; the full browser suite for
broad changes) and list the actual results in the PR. Container checks can't run in those environments; use the
manual full verification before a release. Windows checks were removed (owner, 26 October: Arc
isn't run on Windows for now); re-add a windows-latest job if that changes.

For full verification in GitHub: Actions → Docker image → Run workflow → leave Full verification
checked. A version tag such as v0.1-beta always requires full verification regardless of manual
input. Version names must also be valid container tags. Verification publishes the prepared
image by digest without building it again. No owner server is updated automatically.

Measured previous run 40315984922: image preparation 3.9 minutes, Linux checks 17.1 minutes,
including 9.6 minutes of browser journeys; publication added 0.5 minutes after the gates. The
new main path removes that verification wait, not compilation, runner queueing or registry time.
Actual new publication timing is recorded in READINESS/GitHub evidence; no fixed duration is
promised. Full verified acceptance at 2ba97df (93/93 journeys) remains historical evidence.

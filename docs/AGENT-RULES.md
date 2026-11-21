# Arc: rules for AI agents (all repositories)

The shared rules for every Arc repository (this server, arc-android, arc-apple, arc-kodi, arc-roku,
arc-tv), for Claude and Codex alike. Each repository's own `CLAUDE.md` or `AGENTS.md` repeats the
must-never rules and adds only what's particular to it. Read this at the start of a session. The
owner (GitHub: JonzaUK) drives product vision; agents do most implementation.

## Context and reference documents

- Read the repository's entry instructions (`AGENTS.md` / `CLAUDE.md`), these shared rules and
  applicable directory instructions first. Read each unchanged document once per session;
  re-read after relevant updates or when its contents are no longer available in context.
- Load reference documents only when relevant: setup guidance before environment commands,
  build/test guidance for validation, API guidance for API work, and roadmap/readiness documents
  for planning or release work. Follow explicit reading requirements and linked prerequisites;
  this does not waive rules or required checks.
- Search for relevant sections and symbols before reading large documents or source files.
  Keep tool output to useful excerpts, diffs and check summaries; retain full logs in files when
  needed. Handoffs record the branch/commit, completed work, actual checks and next steps so
  resumed work does not repeat completed investigation.

## Workflow

- Everything goes through pull requests: never push to main, never force push. Work on the branch
  the session assigns (Claude: `claude/...`; Codex: `codex/...`), one agent per branch.
- The owner merges (squash merges), or tells the agent to. After a PR merges, restart the branch
  from the latest main rather than stacking on merged history.
- PRs start no GitHub Actions runs. Before a PR, run the repository's checks yourself and list
  the actual results in the PR, including what was not tried (no TV, no device). Use the
  repository's PR template where it has one.
- Record each change in `docs/changes/YYYY-MM-DD-<slug>.md`: what changed, why, decisions, checks,
  limits, logging. Screenshots go beside it in `docs/changes/YYYY-MM-DD-<slug>/` as JPEGs; link
  that folder from the PR (GitHub's tools here strip image embeds from PR text).
- Commit messages: a summary line, a body saying what changed, and the Co-Authored-By trailer. No
  agent session links in commits, PRs or comments (no `Claude-Session:` or other chat URLs;
  owner, 13 November): Co-Authored-By is the only attribution.
- Keep working without check-ins; stop only for risky or irreversible things. For product
  decisions pick a sensible default that's easy to change and note it in the PR's "Decisions"
  section (what, why, how Plex does it, the alternative). Only when the owner really has to choose,
  open one arc-media-server issue per decision, labelled `owner decision`, with a recommendation.
  #80 is closed history; don't add to it.
- Track work in GitHub issues. App to-do lists (e.g. arc-android#2): tick an item as soon as a PR
  implements it, naming the PR; bugs found later get their own issues. Feature parity across the
  apps is tracked on arc-media-server#476.

## Never

- Never point tests or tools at the owner's server, data, settings, media or keys; use scratch
  servers and generated media. Keys, tokens and signing material never go in git or in chat.
- Never mark hardware, real-device, playback, HDR, Dolby Vision, audio, deployment or sustained-use
  checks done without real evidence, and say what the evidence was.
- Never modify media files: Arc only reads them (no renaming, moving, tagging or writing beside them).
- No plugin system, no Jellyfin-compatible API, no deleting media from the apps (settled).

## Product

- **Plex first:** for every feature, check how Plex does it (on that platform) and match its
  behaviour, defaults and where its settings live, unless there's a clear reason to differ; note
  the difference in the PR.
- **Ease over options:** Apple-level polish; no setting without a reason. Necessary configuration
  lives in the web UI, never config files; keep implementation knobs out of ordinary flows.
- **Reuse before building:** when a feature needs a menu, dialog, screen or logic Arc already has,
  share or extract the existing one rather than writing a second copy.
- **Nothing silently disappears:** unmatched or unplaced items are shown and listed, never hidden.
- **Arc's design** (all TV apps share it; arc-apple is its fullest version): dark only, no UI colour
  (owner, 1 November): chosen and main controls are white with black text, the teal is the logo's
  alone. Artwork-led Home with no banner. The player's controls have the same set and order in
  every app. On TVs, focus is always visible and Back always works.
- **Words:** the same feature has the same name in every app; all user-facing text goes through
  the app's string catalogue, never inline.
- **Episode numbers** (owner, 20 November) read **S1 · E4** everywhere: no leading zeros, no comma,
  and "·" (a middle dot with a space either side) as the only separator, never "•", "-" or ",".
  A special is "Special 3". On a card the line under the show's name is just "S1 · E4": the
  episode title never shares that line (a card that has a line of its own for the title, such
  as Recently Added's, keeps it). Where the episode itself is the subject (the player, the title
  page's resume line, episode lists, play queues) add the title after it: "S1 · E4 · Pilot".
  Buttons too: "Play S1 · E4", "Resume S1 · E4".
- Screenshots use the shared screenshot library (docs/SCREENSHOTS.md), the same titles in every
  app, with TMDB credited; store listings will use licence-checked artwork.
- A show's details come from its library's one metadata provider; gaps are never backfilled from
  the other. Storage order (file numbering) is separate from display order.

## API

- Apps use only `/api/v1`. It's promised stable: the server's `docs/api-v1-contract.txt` lists every
  endpoint, status and field apps may rely on, and a test fails if one disappears or changes type.
  Add fields instead of renaming or removing them. Apps ignore unknown fields.
- If an app needs something v1 lacks, make an additive server PR and link the two PRs.
- Client guide: `docs/CLIENT-INTEGRATION.md`; reference: `docs/API.md`.

## Logging (every repository)

- Log outcomes and failures; detail only at debug. Log a failure once, where it's handled.
  Expected cancellations and empty results aren't errors.
- Never log passwords, keys, tokens, PINs, cast links, full URLs, query strings or request bodies.
- Each PR says what it logs (or why nothing is needed). The server's fuller rules are in its
  `CLAUDE.md` and `docs/LOGGING.md`.

## Working with the owner

- Explain trade-offs in plain language with a recommendation: the owner is a Plex/Jellyfin power
  user, not a Go/React/Kotlin expert.
- Other media servers and apps are references for ideas only: never copy their code, artwork or
  exact designs.
- Licence: AGPL-3.0-only, copyright "The Arc Media Server Authors" (the Apple app's App Store
  exception is decision arc-media-server#477).

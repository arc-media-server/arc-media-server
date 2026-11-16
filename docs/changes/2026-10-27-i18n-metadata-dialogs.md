# Metadata-dialog translation preparation — 27 October 2026

Phase 4c of #122, independent `codex/i18n-metadata-dialogs` branch from main 7b1e4d3.
Choose Artwork, season posters, Fix Match/explanations, skip-marker editing and removable
chips now use 88 bundled English entries in metadata.json. Dynamic names/IDs, provider data,
source-kind/marker enums and API requests are unchanged. Complete per-artwork phrases avoid
building translated sentences from nouns. Rich provider-search explanations use Trans.
No dependencies, translations/picker, schema, layout or product changes. Existing old-English
ams.yaml help remains verbatim; this pass does not silently rewrite help or server messages.
The raw artworkNames compatibility export remains for older callers until final integration.

Actual local verification: production/e2e TypeScript and production/embedded builds passed;
16/16 existing translation/key/plural checks; all-package Go vet and 449/449 Go tests/subtests
(275 top-level, 30 test-bearing packages, six additional packages with no tests). No test skips
or failures. 5/5 unchanged focused browser journeys
passed in 29.978s with no skips/retries/flaky results/suite errors.
Two additional desktop/phone visual journeys passed; four pairs compared against separately
built clean main 7b1e4d3. Artwork is byte-identical at both widths; matching differs at 73
and 65 pixels around small icons/text. Captures inspected; wording/styles unchanged. These
are compared captures, not a claim of four byte-identical pairs.

Temporary stores/generated media/local provider fixtures only. The visual harness normalizes
random temporary file roots in GET presentation only, waits for artwork requests and uses
viewport captures for dialogs; earlier full-page scrolling/hover/loading differences remain
in the evidence as harness diagnostics. Initial scope/type/audit errors were fixed before
these checks. No existing browser test or selector changed. No owner/production data, real
keys, Unraid, persistent server, full-suite/race/container/device/deployment acceptance claim.

Evidence: /workspace/arc-cloud-evidence/i18n-remaining/metadata-*.
Owner requested all remaining areas as separate unmerged PRs for batch review; keep #122 open.
Detail/order dialogs, bulk/preplay controls, Player, Settings, management and final integration
have separate branches/PRs. No main push/merge, rebase or force-push; no hosted-CI polling.

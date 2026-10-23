# Scan Activity clarity

Owner approved, 23 October 2026. Tracked in #167.

1. Keep one task per library; show the actual active movie/series/file and stage.
   Matching is concurrent: report bounded live operations, not a single last title.
2. Distinguish file discovery/reuse, provider matching, metadata refresh, episode
   linking/saving and FFprobe inspection. Background format analysis remains separate.
   Name a provider only at its actual call site; do not call artwork references downloads.
3. Use known completed counts and indeterminate progress for unknown totals. Completed
   scan summaries distinguish reused details, inspection attempts/errors, newly identified
   titles and unmatched/unrecognised files. Do not infer counts of updated titles.
4. Keep Activity readable on phones with wrapping title/stage text, current operations
   expandable when several run, and optional filename details. Show filenames, not absolute
   media roots. Keep detailed provider diagnostics in existing redacted logs.
5. Preserve scanner concurrency, schedules, matching/probing rules and read-only media.
   Reporting must not trigger new probes/providers or per-file queries on unchanged scans.
6. Verify actual held provider/prober operations, concurrent isolation/snapshot ownership,
   completion/cancellation cleanup, cached rescan counts, and analysis identity. Check
   affected Go packages/race plus build/app/e2e TypeScript and focused desktop/phone Activity
   journeys. Use only temporary stores/generated clips/local providers; record limitations.

Implemented with bounded worker-scoped Activity operations, scanner/provider/inspection hooks,
additive status counts and expandable Activity rendering. Matching/probing/scheduling rules are
unchanged. Actual results and initial failures are recorded in READINESS and cloud evidence.

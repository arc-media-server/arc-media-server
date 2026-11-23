# Logging and stall diagnostics

Shared PR rules are in `CLAUDE.md` and `.github/pull_request_template.md`. Review observability
for each change; do not add logs merely to satisfy a checkbox. Existing features are not all
retrofitted by introducing this rule.

## Detailed logging

Settings → Maintenance & diagnostics enables debug records for **two hours**. The UI shows the
server-provided expiry time. It can be stopped early; enabling it again renews the full window.
A server restart returns to normal logging unless explicitly started with `-debug` (which starts
a new two-hour window). Retained logs still survive restart when persistent storage is available.
The existing retention limit stays three 2 MB files: a longer capture window does not guarantee
that a busy two-hour period fits. Download diagnostics promptly after reproducing a fault.

## Finding unfinished library/watch-state requests

For library lists/scan status, Home Continue Watching and watchlist/watched/progress writes:

- At ten seconds, a still-running request can emit one warning with a server-local request ID,
  registered route template, stage/elapsed time, optional numeric library ID and database pool
  pressure. Request IDs correlate with completion logs. A slow request is not proof of deadlock.
- Library stages distinguish database loading, title counts, scanner status and response writing.
  Watch-state stages identify the save; authentication/title-access stages are also distinguishable.
  Some remaining work (including Home calculations) has only a generic handler stage.
- A maximum of 64 requests are tracked simultaneously; additional tracking is omitted, not queued.
  Diagnostics report the omission count. These are observation limits, not concurrency limits.
- Unfinished-request warnings are capped at five per minute across the server, once per tracked
  request. Targeted GET completions also warn after ten seconds; the existing two-second mutation
  warning threshold remains. This cap is not a global log cap.
- Successful requests do not gain new info records. Frequent Activity/log/resource polls, streaming
  and downloads are excluded from this tracking. Fast tracked requests allocate a small record and
  stopped timer; no per-request goroutine runs unless the threshold timer fires.

Database pool counters are instantaneous or cumulative, as indicated by their names. They do not
attribute SQL time to an individual request or identify a transaction owner. This change does not
wrap SQL transactions or change queue/admission/cancellation behaviour. On-demand stacks help
investigate the actual wait without introducing per-query logging into every write.

## Admin diagnostics download

The existing admin-only ZIP adds:

- `requests.json`: up to 64 active tracked requests with elapsed stage times, the cap and cumulative
  tracking omissions. Completed requests are removed. No raw URLs, bodies or title names.
- `goroutines.txt`: grouped Go call stacks captured only for this download, with function names and
  source paths but without function argument values. Output is capped at 512 KiB plus a truncation
  marker and passed through the existing credential redactor.

`manifest.json` and `logs.jsonl` remain. This is not a database/settings dump or a memory/core dump.
A snapshot is evidence at capture time, not a recording of earlier waits. Stack collection has an
on-demand runtime cost; it is never performed for routine logging or every slow request. Runtime
stack grouping may not establish the exact lock owner, and request tracking does not cover every
API or background job. No public profiling endpoint is enabled.

## Current incident boundary

The owner's 29 October diagnostics showed multi-minute write stalls and a later unanswered library
request while other APIs remained responsive. They did not contain stacks or active request stages.
IMDb ingestion was separately paused in PR #280. These logging changes improve the next capture;
they do not prove or fix the original library hang, database corruption or an IMDb root cause.

## Logs sent by Arc apps

The Dashboard's device log view displays user-approved diagnostic events from Arc apps.
See [Sending device logs](CLIENT-LOGS.md) for the wire format, privacy rules and retention.

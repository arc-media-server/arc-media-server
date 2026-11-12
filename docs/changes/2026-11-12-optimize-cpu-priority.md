# Let interactive work take CPU priority over Optimize (#193)

The owner authorized the follow-up to the controlled resource assessment in PR #438. Explicit
Optimize used most of two available cores and increased live-conversion plan readiness from
1.5–1.7 seconds alone to 2.7–3.3 seconds alongside it. Those were synthetic cloud HTTP results,
not owner-server or decoded playback measurements.

Give only the Optimize FFmpeg child a best-effort background CPU priority. Linux lowers the
leader and existing worker threads to nice +10, with new workers inheriting it; macOS uses
process nice +10; Windows uses Below Normal. Preserve an already lower priority. Arc and live
conversions keep their normal priorities. On other platforms or host denial, preserve working
conversion behavior and warn once per optimizer lifetime. No new setting or dependency.

Keep the existing serial queue, attempt/source identity, output formats, cancellation, shutdown,
restart and completion checks. No pause/restart loop or discarded encoding progress. This does
not impose an I/O/storage quota, change admission limits or include pending #372's safeguards.

A real child/thread regression failed before the change, then passed. It checks all Linux
worker priorities and preservation of the parent's and an already lower child's priorities.
A portable injected-denial test checks successful completion of two copies and one bounded
warning. Existing conversion/source/audio/restart/cleanup checks remain in the affected suite.
Actual final Go/race/cross-compilation and repeated FFmpeg measurement results are recorded in
the PR. Windows/macOS runtime behavior, GPU/NAS/physical playback and sustained resource impact
remain separate acceptance; Linux scheduling is a best-effort hint, not a latency guarantee.

Repeating the same constrained load run with this change produced live-plan readiness of
1.820/1.819 seconds alongside Optimize, versus 3.330/2.734 seconds before. Combined browse p95
was 10.29ms versus 17.43ms. All 888 browse responses, 40 original range reads and six HLS segment
reads succeeded; expected second-live 503 rejections and cancellation cleanup remained correct.
Process inspection found Optimize worker threads at nice +10 and live workers at nice 0. These
are directional two-round RAM-backed HTTP measurements, not statistical/decoded-player/NAS/GPU
or owner-hardware acceptance. Total CPU remained busy; the hint changes allocation. Background
completion can take longer while foreground work competes.

Logging: one warning per optimizer lifetime if the priority hint fails, with only the OS error.
No success, per-file/progress-loop, URL, token, credential or media-path logging added. Normal
job failures retain existing diagnostics. Source media is read-only; all verification uses
generated files and temporary stores, without production or Unraid access or a persistent server.

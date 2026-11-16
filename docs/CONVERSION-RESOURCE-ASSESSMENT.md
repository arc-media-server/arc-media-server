# Conversion resource assessment (#193)

Controlled cloud assessment, 12 November 2026. Main baseline:
`2d8d4ea4aefe5ee4974b961e63b184e3a76b59e3`. This is a measurement report, not a change to
conversion scheduling, storage policy or any API. Pending review fixes are identified separately.

## What current code already does

- Live conversion has ahead-of-player pacing: default pause ahead 120 seconds, resume one
  six-second segment closer. See `internal/transcode/throttle.go`. The original #193 audit's
  missing-throttle statement describes its older baseline, not current main.
- Live video-conversion admission uses the configured limit; it does not include explicit
  Optimize jobs. Explicit optimization is a separate serial queue, outside optional maintenance.
  See `internal/transcode/transcode.go`, `internal/optimize/optimize.go` and `cmd/ams/main.go`.
- Optional maintenance yields to playback/foreground work. Explicit Optimize does not implement
  that automatic pause. Its continued work during playback is current policy, not a newly
  reproduced scheduler bug. See `internal/background/background.go` and `BACKGROUND-WORK-PLAN.md`.
- Live session cleanup and optimized partial/restart cleanup already exist. Completed optimized
  copies have explicit size reporting/deletion, not automatic eviction under a shared budget.
- PR #372 adds live-conversion free-space admission/monitoring and opt-in rolling HLS cleanup.
  Those are pending changes, not main's safeguards or an optimized-copy storage budget.

## Method

Prepared Go 1.27.0, Node 24.19.0/npm 11.9.0 and Jellyfin FFmpeg 8.1.3. No tools or dependencies
installed. The existing browser-test supervisor built/runs the real embedded server with a new
temporary SQLite store and local provider stand-ins; the matching main UI build was reused.
No browser/production service, owner settings/media/keys, Unraid, native repositories or GPU used.

Generated a 180-second, 1920x 1080, 24fps testsrc2 H.264/AAC MP 4: 262,841,077 bytes. It was created
only for this test. Source generation used libx264 ultrafast/CRF 25 and two threads; Optimize used
Arc's unchanged faster/CRF 21 presets. All generated source/output/DB bytes stayed in `/tmp`,
which is RAM-backed here. The workspace filesystem was already 98% used, so it was deliberately
not used for generated media or pressure simulation. No filesystem was filled.

The container reports an eight-GiB memory ceiling and a two-CPU cgroup quota. Arc and its
children were restricted to CPUs 0/1; GOMAXPROCS=2 was set for this experiment. The HTTP driver ran
in the same constrained container. This is a small one-title library, not a large-library test.

Two loaded rounds: idle 8s → Optimize 8s → Optimize plus original range reads 8s → Optimize plus
720p live conversion 12s → cancellation and idle 8s. Queue 720p and 1080p copies to verify only one
Optimize worker runs while the other remains queued. Mark the client playing through the normal
progress API. Configure one live conversion; attempt another at a different source offset.

Two later control rounds: original reads 8s → 720p live conversion 12s, with no Optimize job.
The identical generated source was reused in fresh temporary stores. Fixed ordering and cache
warmth remain limitations; two repetitions are directional observations, not a statistical study.

Every roughly 400ms cycle requests libraries, sorted items, library facets and Continue Watching.
Record each complete HTTP response time. Sample Arc/FFmpeg process CPU/RSS, process states,
`/proc` I/O counters and generated-file sizes. CPU 100% means one fully used core; RSS sums may
count shared pages twice and exclude tmpfs/cache memory. CPU estimates use sampled process ticks.

Transport is a **synthetic HTTP consumer**, not a decoded player: original reads request 256KiB
ranges; converted reads poll the playlist and fetch the next available segment at six-second
intervals. No video/audio decoding, HLS initialization playback or real-time source-bitrate
consumption is asserted. Playback-plan readiness below is the HTTP start response, not display
of a first frame. RAM storage and cache mean these counters do not measure NAS/disk throughput,
seek pressure or physical I/O latency. No smoothness, HDR, audio-output or owner-hardware claim.

## Results

Browse times combine the four endpoints and both repetitions. They are not an application SLO.
Raw per-endpoint samples remain available; these are whole-response times, not only server SQL.

| Scenario | Browse requests | p50 / p95, ms | Mean Arc+FFmpeg CPU | Peak summed RSS, MiB |
| --- | ---: | ---: | ---: | ---: |
| Idle before work | 160 | 2.93 / 8.22 | 1.8% | 27.2 |
| Optimize | 164 | 3.44 / 13.23 | 186.0% | 226.8 |
| Original range reads alone | 160 | 2.90 / 5.55 | 2.1% | 27.1 |
| Optimize + original range reads | 164 | 3.50 / 13.61 | 188.3% | 228.4 |
| Live conversion alone | 248 | 4.22 / 11.59 | 188.3% | 197.2 |
| Optimize + live conversion | 240 | 4.01 / 17.43 | 193.0% | 398.5 |
| Idle after cancellation | 160 | 2.86 / 9.14 | 1.9% | 27.4 |

- **1,296/1,296 browse requests returned 200**, with no unexpected transport errors. Worst single
  browse response across phases was 58.85ms. Small responses and a tiny warm catalog limit this.
- **81 original range responses returned 206; ten HLS segment responses returned 200.** All requested
  media reads succeeded; this does not prove playback consumption/decoding or sustained throughput.
- Live playback plan readiness was 1,684.6/1,518.2ms alone and 3,330.2/2,734.1ms alongside Optimize.
  The observed extra startup time is consistent with CPU competition. This does not isolate all
  causal factors or predict the owner's Quick Sync hardware behavior.
- One Optimize process remained active during direct/live phases, with the second copy queued.
  Combined live/Optimize work reached two FFmpeg processes despite max_transcodes=1, as designed.
  A second live conversion was correctly rejected with 503 in both loaded rounds.
- Explicit cancellation removed both optimized requests, stopped FFmpeg and left zero optimized
  and live-output bytes. Measured Optimize delete-to-clean times were 14.3/13.5ms. These are sample
  observations, not a promised shutdown deadline or proof of blocked-filesystem cancellation.
- Peak observed optimized output was 18.25MiB; live output peaked 6.66MiB under contention and 16MiB
  in unloaded controls. These partial-run sizes are not a quota or completed-copy size forecast.

## Failure and safeguard checks

On the same disposable server, make only its empty optimized-output directory unwritable
(mode 0500), then request Optimize. FFmpeg fails normally; the job becomes visibly failed with a
permission-denied reason and zero partial bytes. Restore permissions, retry, observe converting,
then delete it: child exits and outputs/records clean up. The source directory is not altered.
This is a real permission/write failure, **not an actual ENOSPC event**.

Fresh focused current-main tests: **11 passed, zero failed/skipped**:

- Live capacity and failed-start slot release: TestConcurrentConversionLimit,
  TestFailedConversionStartReleasesCapacity.
- Player-aware pacing: TestConversionWaitsForThePlayer,
  TestThrottlingResumesWithinOneSegment (focused fixtures, not the loaded HTTP transport run).
- Recovery/cleanup: TestConversionStorageRecovery,
  TestShutdownStopsConversionAndRestartClearsOutput,
  TestOptimizedRestartPrunesOnlyOwnedOutput, TestInterruptedOptimizationResumes.
- Optional-work preemption: TestForegroundAndSettingsCancelLeaseAndResume,
  TestUpkeepYieldsAndReportsInterruptedWork, TestManualUpkeepQueuedStatusAndPreemption.

Separately, on pending PR #372 at `ed0d7d5`, **two tests passed, zero failed/skipped**:
TestConversionFreeSpaceAdmissionAndStop injects free space below 512MiB and checks rejected
admission, released capacity and a diagnosable active-session stop; TestRollingHLSRealConvertedSegments
checks consumed-segment eviction using real Jellyfin FFmpeg 8 output. This does not establish that
these protections are merged, that every generated-asset class has a budget, or that actual
disk exhaustion was rehearsed. No race/full suite/browser suite was run for this assessment.

## Recommendation and remaining acceptance

The measured cost is CPU saturation and slower live-start readiness; browsing remained responsive
in this small controlled run. Prioritize assessing how explicit Optimize should yield CPU to an
interactive live conversion. Reuse existing worker/process lifecycle and keep the behavior
cross-platform; avoid a second queue or ordinary worker-count settings. Preserve correct cancel,
restart and job identity, and do not discard/restart expensive work repeatedly just to pause it.
No scheduling change is made or approved by this report.

Do not invent a unified quota/eviction policy from RAM-backed measurements. Review/merge the
existing #372 protections first. Before choosing additional policy, measure representative
owner-server CPU/GPU, physical I/O and mixed playback, and classify generated assets separately:
active HLS, retained optimized copies, regenerable previews/artwork cache and irreplaceable uploads.
Source media and uploaded artwork must not become quota-eviction targets.

Keep #193 open. The cloud assessment completes an initial controlled measurement, not the
owner-server/disk-pressure/device acceptance or implementation of a new policy. The next code
change, if authorized, should have an explicit live-start/throughput target and regressions for
pause/resume, cancellation, failed admission, restart and bounded logging. Device acceptance
and sustained NAS measurements remain separate.

## Reproduction and evidence

Activate `/workspace/arc-cloud-evidence/review-350/activate.sh` for the prepared compiler/media tools.
The two drivers, raw request/resource samples, summaries and logs are retained under
`/workspace/arc-cloud-evidence/resource-193/`: `measure.mjs`, `control.mjs`, `summarize.py`,
`summary.json`, per-phase JSON, both result.json files and focused Go JSONL.

Main command: `node /workspace/arc-cloud-evidence/resource-193/measure.mjs`; the control driver
uses a preserved generated fixture from that run. Both manage their own temporary supervisor
and shut it down. Initial harness run incorrectly expected 202 for Optimize's actual 200 response;
it stopped after idle/request admission, retained in initial-harness. The corrected complete
runs exited 0. No product error or performance improvement is inferred from that harness mistake.
All final generated media, temporary databases and the saved fixture link were removed.

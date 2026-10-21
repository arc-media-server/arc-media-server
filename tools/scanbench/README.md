# Timing scans

Times a first scan of a big movie library without touching TMDB, with slow disks simulated.
Use it before and after changing the scanner (milestone 20, #128).

```sh
# 1. A library of N films, each a copy of one small real video
node tools/scanbench/make-library.cjs "../ams-test-media/Movies/The Matrix (1999)/The Matrix (1999).mp4" ../bench-films 600
# 2. A stand-in TMDB answering every request after DELAY ms (about a real round trip)
DELAY=200 node tools/scanbench/tmdb-stub.cjs
# 3. A fresh Arc pointed at it, with 300 ms added to every file read (a busy hard drive)
AMS_LISTEN=127.0.0.1:8198 AMS_TMDB_URL=http://127.0.0.1:8305 AMS_TMDB_API_KEY=x AMS_PROBE_DELAY_MS=300 \
  go run ./cmd/ams -data ../bench-data -config none
# 4. Time it: 10/50/90/100% matched and finished (or preview first, then add, counting TMDB requests)
node tools/scanbench/time-scan.cjs http://127.0.0.1:8198 /full/path/to/bench-films 600
node tools/scanbench/preview-then-add.cjs http://127.0.0.1:8198 /full/path/to/bench-films 600
```

Results on the development PC (600 films, 200 ms TMDB, 300 ms per file read):

| | 10% matched | 50% | all matched | scan finished |
|---|---|---|---|---|
| Before milestone 20 | 226 s | 326 s | 449 s | 449 s |
| After | 6 s | 22 s | 43 s | 110 s |
| After, added from a preview | | | 23 s | 90 s (half the TMDB requests) |

The historical preview row starts its clock **after** Preview. It counts requests made during
the subsequent scan, not the entire preview-and-add workflow. Do not use it as an end-to-end
comparison or evidence that Preview halves total requests.

## Current workflow investigation (21 October 2026, #145)

`node tools/scanbench/review.mjs` builds the embedded server, generates 120 one-second H.264
clips, starts a local delayed TMDB stand-in and runs two independent servers with fresh
temporary stores. Go and FFmpeg must be on PATH; build the production UI first if it changed.
It removes inherited `AMS_*` settings, never targets an existing server/data directory, uses
only generated media, and cleans up its exact temporary directory. No real API keys are used.

The stand-in responds after 200ms; each real ffprobe call has 300ms artificial delay. Current
product baseline: `c22b671`, Windows amd64, Ryzen 7 7700X. All times start before preview/add:

| Flow | Preview | First matched | All matched | Scan finished | Provider requests |
| --- | ---: | ---: | ---: | ---: | ---: |
| Direct API add | none | 0.86s | 8.88s | 21.97s | 120 searches + 120 details |
| Complete Preview, then Add | 12.38s | 12.87s | 16.96s | 30.06s | 120 searches + 240 details |

The UI permits Add before Preview finishes; this measurement tests the completed-preview
path, not a mandatory wait. At that baseline Movie Preview fetched full details but retained
only the match candidate; the scan fetched details again. The implementation below addresses it.

For store/grouping work separately, run:

```sh
go test ./internal/scanner -run '^$' -bench BenchmarkScanWork -benchtime=1x -count=1 -benchmem
go test ./internal/scanner -run '^$' -bench 'BenchmarkScanWork/movies/(unchanged|add-one)$' -benchtime=1x -count=3 -benchmem
```

The benchmark uses 1,800 synthetic files with providers disabled and probes pre-seeded.
The TV layout is 30 shows with 60 episodes each. Fixture setup, initial seeding for rescan
cases and adding the extra synthetic file are outside the measured scan. The one-add case
measures processing the addition, not a filesystem event/watch delay or real new-file probe.

| Library | First grouping | Unchanged rescan | One added file |
| --- | ---: | ---: | ---: |
| 1,800 movies | 3.38s | 2.99s | 2.86s |
| 30 x 60 TV episodes | 1.61s | 0.735s | 0.762s |

Three fresh repeated movie trials gave 2.848–2.875s unchanged and 2.872–2.889s with one added.
This confirms that one addition still pays almost the whole-library scan cost. Allocation
figures are cumulative allocations per scan, not peak RAM. These runs do not measure physical
HDD/SMB behavior, live providers, Plex performance, real-device playback or the owner's Unraid.

## Implemented improvements (#145, 21 October 2026)

The same Windows machine, fixtures and commands were used after the changes. Timed runs
below were sequential, without another build/browser suite running alongside them.

| Flow (120 movies) | Preview | First matched | All matched | Scan finished | Provider requests |
| --- | ---: | ---: | ---: | ---: | ---: |
| Direct API add | none | 0.50s | 8.54s | 21.66s | 120 searches + 120 details |
| Complete Preview, then Add | 12.51s | 12.53s | 12.89s | 26.08s | 120 searches + 120 details |

All times include Preview when used. Reuse removes the 120 repeated detail requests:
the completed-preview workflow is 3.98s faster here, but direct addition is still faster.
First-match time improves; total direct-import completion is essentially unchanged because
probing follows matching and dominates this fixture. Matching visibly sooner is distinct from
finishing or proving playable media sooner; this tool measures matched items and scan completion.

Three fresh synthetic rescan trials (`-count=3`):

| Library | Unchanged full rescan | Scoped addition |
| --- | ---: | ---: |
| 1,800 movies | 0.835–0.853s | 0.00169–0.00242s |
| 30 x 60 TV episodes | 0.135–0.265s | 0.00847–0.01000s |

A separate single cold-grouping trial measured 1.27s for movies (baseline 3.38s) and 1.36s
for TV (baseline 1.61s). The TV numbers include the follow-up fix that shares one season-folder
artwork enumeration across its episodes; enumerating it for every episode was slower and was
corrected before shipping. Cold figures are single trials; repeat ranges above show variance.

The scoped movie addition visits one movie folder; the TV addition visits 61 files of its
existing show, preserving the original library root for episode parsing. The full hourly/manual
reconciliation remains necessary. These numbers exclude the ten-second watcher settling wait,
provider calls, actual new-file probing and fixture setup. They are not end-to-end addition times.

```sh
go test ./internal/scanner -run '^$' -bench 'BenchmarkScanWork/(movies|shows)/(unchanged|targeted-add)$' -benchtime=1x -count=3 -benchmem
```

Correctness fixtures exercise discovery/matching overlap, scoped Unicode missing-file sweeps,
split show folders, large change batches, changes queued during scan completion, and TV source
switching with stable episode/file IDs, edits, resume and custom locked placements. A delayed
alternate-order endpoint proves essential episodes are linked first; four temporary failures
then recover automatically without re-downloading cached show/season details. This establishes
fixture behavior, not a live provider success rate or sustained Unraid playback responsiveness.

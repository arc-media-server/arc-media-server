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

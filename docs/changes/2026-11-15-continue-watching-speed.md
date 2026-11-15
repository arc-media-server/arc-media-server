# Continue Watching: read recent history only, batch the next-episode check

After #470 and #471, Continue Watching was the slowest Recommended row on the owner's Series
library (343–399 ms). It's also the first row on Home. Up Next, which finds the next episode
of each show being watched, did three slow things:

- **Finding the shows being watched** grouped the profile's whole watch history by show to
  pick the 40 most recent. It now reads the history newest first through the existing
  `user_item_data_recent` index and stops once it has 40 shows. On the 100,000-episode test
  library this went from about 117 ms to under 1 ms.
- **Checking each show's next episode** was one query per show (up to 40). The first eight
  episodes after the last watched one are now checked for every show in one query. A show whose
  next episode is further on (gaps, missing files) probes its own growing windows of 32, then
  128. Each episode is now looked up by id rather than by scanning all its show's episodes.
- **Loading the chosen episodes** was one query each; now it's one.

The page is the same: same shows, same next episodes, same order and the same 20-show cap.
Ties in "most recently watched" at the 40th show were never ordered before and still aren't.

Validation: a new store test compares the shows found with the old grouping query across
profiles, an age limit, a library limit and different limits, and checks the plan reads the
recent-history index without sorting (it fails if the order is reversed). Codex's Up Next window
tests (gaps, missing and watched successors, in-progress, custom orders, specials, dismissed,
unmapped) pass unchanged. On Codex's 100,000-episode measurement, the Continue step went from
216–223 ms to 67–74 ms (1,000 shows × 100) and 282–359 ms to 121 ms (200 × 500), two runs each
against main. `go vet ./...`, `go test ./...` (all pass except `internal/transcode` tests that
need FFmpeg features this machine lacks, which fail the same way on main), race runs of the
api and store tests and `git diff --check` passed.

Logging: no change; the existing Recommended debug timings show the `continue` step.

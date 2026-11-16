# Avoid whole-show watch-state reads when finding Up Next

Continue Watching's Up Next path previously loaded file availability for every episode of
up to 40 candidate shows, then fetched complete watch-state records for every entry in each
show's display order. Only the first available, unwatched successor was needed.

Keep the existing shared display traversal and candidate limits. After locating the last
watched episode, inspect 128 ordered entries at a time. A single indexed query checks their
parent identity, present files, watched status and resume position; stop at the first eligible
successor, including an in-progress successor that belongs in Continue rather than Up Next.
Do not cap the search: subsequent windows still find older/distant successors after missing
or watched entries. Custom orders, inline specials, dismissals and unmapped last episodes
keep their existing semantics. No persistent state/cache or API changes.

When there are no inline special placements, DisplayEntries now partitions regular entries
and Specials directly instead of building unused identity/target/placement maps. This is the
same shared browse/playback traversal; repeated regular identities and special ordering remain.

Focused tests cover successors beyond the first window, missing/already-watched entries,
custom orders, inline specials, in-progress successors, dismissals and unmapped last entries.
Existing special traversal and access tests remain relevant. Existing Continue/request debug
and boundary error logging is retained; no per-episode or info-level records are added.

Controlled measurements against main d58a433, 10,000 movies +100,000 episodes, fixed history
and matching daily selections: long-show Continue stage median 491→388 ms; complete Series
page 1,326→1,205 ms. For 1,000×100, Continue was essentially unchanged (283→285 ms; whole page
1,243→1,257 ms). Series allocation fell 55.8→21.5 MB/request on 200×500 and 12.7→8.0 MB on
1,000×100. Complete JSON snapshots matched for both Series and Movies, both shapes.

An initial 32-entry window increased the broad-shape Continue median to 311 ms; the final
128-entry window avoids those extra round trips. All preliminary samples are retained outside
the repository. First plus five repeats per library, run sequentially; not OS-cache-cold,
network, Kodi or owner-server timings. A few milliseconds of cgroup throttling occurred in the
final run; no claim that timing variance is eliminated or every library becomes faster.

Validation: `go vet ./...`, full `go test ./...` (37 packages passed; 7 without runnable tests),
and focused API/store race checks passed (72.386s/11.295s), including successor-window,
Continue filtering, special traversal, repeated identities and profile watch-state scenarios.
`git diff --check` passed. Web unchanged; no browser suite or deployment acceptance claimed.

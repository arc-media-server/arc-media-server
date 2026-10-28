# Faster search

The owner asked whether search could be faster. Measured on a benchmark library (2,000 movies,
400 shows with 8,000 episodes, 60,000 people with 40 credits per title, two other names each;
`go test ./internal/api -run '^$' -bench BenchmarkSearch`), each search took 165–270 ms on the
cloud machine before any drawing in the browser. Now it takes 45–75 ms.

Where the time went, and what changed:
- **Every person's name was reloaded and normalized on every keystroke** (about half the time).
  The server now keeps the cast and crew names in memory, already normalized
  (`internal/api/searchindex.go`). It reloads when people are added or removed, and every five
  minutes so renamed people are picked up.
- **Present credits were counted for every person who matched**, thousands for short queries.
  Matches are now ranked by score and by how many credits they have in all, and only the best
  200 have their present credits counted (30 are shown).
- **Every movie and show was loaded with all its details just to compare names.** Search now
  loads only the names (`store.SearchTitleNames`) and full details for the at most 120 titles
  it shows.
- **The typo-tolerant comparison** split the query and allocated on every comparison. A
  `search.Matcher` splits the query once, and the edit distance works on the stack.
- The sidebar waits 150 ms after typing stops before searching, instead of 250 ms.

Results are unchanged: the new `TestSearchResults` (typos, other names, a profile's hidden
labels, titles without files, the busiest person first) passes against both the old and the new
code.

Not done: keeping the previous results on screen while new ones load, and cancelling searches
that are out of date. The search journeys deliberately check that a pending search never shows
the previous query's results under the new heading, and that a late reply to an old search is
handled cleanly.

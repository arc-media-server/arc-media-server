# Share stream traversal across library facets

The owner reported a Series facets request taking 1.823 seconds. The endpoint independently
joined titles/episodes/files/streams for video codecs, dynamic ranges, audio codecs and channels.
Its combined own-file/child-file OR join also obscured indexed identity paths.

Collect accessible titles once, join their own and child files through separate indexed paths,
and materialize present streams once for all four media facet aggregates. Count each title
once per value, preserve value ordering and include own files on series as before. Genre,
label, language, decade, unmatched and movie-resolution behavior stays unchanged. No persistent
cache, migration, dependency or API change.

Existing media facet/access/counting tests plus focused own-file, inherited-label, missing-file
and denied-library cases cover the shared pass. The opt-in full-handler measurement uses
10,000 movies and 100,000 episodes and logs response hashes for equivalence checking:

`ARC_RUN_FACETS_SCALE=1 go test ./internal/api -run '^TestLibraryFacetScaleMeasurements$' -count=1 -v`

Existing request timing/error logging covers this synchronous operation; no success/info floods
or per-file records are added. Only disposable synthetic stores are used.

Same-day, sequential first plus three repeats against main d58a433: Series facets median
2,057→1,254 ms and Movies 1,183→569 ms. All response hashes matched across versions and
repeats. A preliminary pass before excluding unused subtitle streams measured 1,229/468 ms;
those samples are retained, not substituted for the final implementation's results. The
fixture contains 1,000 shows ×100 episodes and 10,000 movies; no real storage, network or
owner-server performance claim. No other benchmarks/builds ran during measurements.

The shared stream relation is temporary and scales with the requested library's stream count;
it is released with the query, not retained as a cache. This trades temporary materialization
for repeated joins. Peak memory/temp-storage behavior on the owner's deployment was not measured.

Validation: `go vet ./...`, full `go test ./...` (37 packages passed; 7 without runnable tests),
focused facet/filter/access store race checks (19.037s), the opt-in full-handler scale test
and `git diff --check` passed. A new test initially assumed episode labels were independent;
source/baseline rules use inherited parent labels, and the corrected test preserves that rule.
Web unchanged; no browser suite or production/device acceptance is claimed. The broader
investigation and still-outstanding query costs are recorded in docs/LIBRARY-PERFORMANCE.md.

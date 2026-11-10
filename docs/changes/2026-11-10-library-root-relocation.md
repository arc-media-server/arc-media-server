# Retain per-file state when a library root moves

Replacing a library folder previously deleted its old media_files rows. A rescan
then recreated files without their probe data, edited markers, merge/split links
or optimized-copy provenance.

For a single absolute root replaced with one disjoint root, inspect the matching
relative file paths before opening the write transaction. Only regular files with
the same positive saved size and second-resolution modification time are mapped.
The transaction rechecks the old roots, saved revision and destination uniqueness.
It updates existing file IDs, preserving their dependent data. Completed optimized
copies also require matching nanosecond provenance; queued/in-flight attempts and
copies with incompatible provenance are revoked.

Other removed paths are marked missing rather than deleted. Nested narrowing,
multi-root changes, different revisions, unavailable files and existing target
identities are not guessed. Filesystem checks do not run under the database writer;
no media is read, moved or written. A later normal scan discovers local artwork
and themes at their new paths. This does not remap Other videos folder-collection
keys or cryptographically prove content equality, and real NAS/Windows moves have
not been tested. Normal explicit trash removal remains a separate operation.

Temporary-file regressions reproduced identity deletion on both UpdateLibrary and
the atomic SaveLibrarySettings path. Tests retain probes, split links, manual
markers and matching optimized provenance, reject stale snapshots and prevent old
optimization attempts publishing after relocation. No per-file/info logging or
raw paths were added; existing library-request error diagnostics remain responsible.

Part of #350. The shared store transaction extraction/settings writer comes from
#375; preserve that PR's API-side library lease and #365's catalog exclusion when
integrating. No API, schema, dependency or frontend changes.

Validation: go vet ./..., go test ./... and
 go test -race -timeout 30m ./internal/store passed (257.387s).
Initial test-fixture mistakes (private helper name, and expecting one optimized
DTO despite two split-item links) are retained separately from the genuine red
deletion regressions; final focused checks passed.

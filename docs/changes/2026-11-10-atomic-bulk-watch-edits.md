# Apply bulk metadata and watched-state edits atomically

A later watch-state failure could return 500 after committing metadata changes,
earlier watched states and watchlist removal. Resolve all selected episode IDs
before writing, then commit metadata, rating age, watch state and movie-watchlist
removal in one transaction. SetPlayed shares its existing SQL with the bulk writer;
watch counts retain the existing idempotent behavior.

A second reproduced failure occurred while synchronizing a bulk content rating's
age after committing its metadata. Write the rating and derived age together.

Red regression tests injected a late watch-write failure and an age-write failure;
both previously left partial changes. Green tests verify complete rollback and a
successful retry applying the full edit exactly once. Existing request failure
logging handles transaction errors, with no new body/SQL/secret logging or loops.
No media, schema, dependencies or frontend changes. Part of #350. Preserve #377's
separate bounded/root-owned refresh admission when merging bulk.go; refresh still
starts only after a successful commit.

Validation: go run ./tools/apidoc, go vet ./..., go test ./... and API race
passed (244.938s). The usage-limit pause interrupted the subsequent store race;
its separate resumed result is recorded below. The earlier interrupted log
is retained, not counted as a pass.
The resumed store race passed (239.803s). API schema/contract regeneration produced
no changes.

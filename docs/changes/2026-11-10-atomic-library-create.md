# #350: validate and publish complete library configuration

The real library route reproduced invalid collection thresholds, language and grid options returning 201 and silently falling back to defaults. A temporary database trigger also reproduced configuration-write failure returning 500 after the core library had already committed. Validate all options first using the shared library validator, then publish core fields, roots, provider and scalar controls in one store transaction.

The configured-creation path rechecks current case-insensitive name ownership and overlapping root ownership in that transaction, so concurrent API creations cannot both claim a name/root. Filesystem readability/normalization checks remain outside the writer transaction. Existing legacy SyncLibrary/CreateLibrary helpers keep their existing migration/fixture behavior; the normal endpoint uses the complete configured method.

Compute the existing defaults before saving: TMDB file order by default, collection threshold 2, server rating region, TV intro/credit controls, movie credits and neither for Other videos. Preserve explicit applicable overrides. Notify watchers, wake requested thumbnail work and start scanning only after commit. The response uses the committed values and stable root order, without another fallible post-commit settings write/read.

Tests reproduce ignored invalid options and partial configuration, verify complete values before notifications, all three media-type defaults, full store-option round trips, late root-insert rollback and concurrent name/root conflicts. A small test-fixture extraction provides a known temporary DB path for controlled trigger injection; default security fixtures behave the same. No owner data, provider keys, media mutation or persistent server.

No API field/type, schema or frontend change. Preserve #375's separate atomic edit/scanner-work reservation and #389's typed input-error classification when merging the shared validator/handler. This change concerns creation, not the separate library-root-move identity finding.

Logging records one committed library outcome with its ID, kind and folder count, not submitted names/paths or a premature success. No per-file/info flood, raw URLs, keys, tokens, bodies or SQL arguments are added. Existing scan outcomes remain responsible for subsequent work.

Validation: ignored-option and partial-write regressions failed before the fix; focused rollback/default/ownership tests, API-doc generation, Go vet, full Go suite and API/store race checks passed (API 259.791s; store 241.472s). Generated API docs and the v1 contract have no diff. No browser/physical deployment acceptance claimed.

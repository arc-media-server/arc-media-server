# Download source identity and catalog ownership (#62, D1)

Bind private download provenance to media_files.cache_key; guard preparation before queue capture
through compare-and-set publication. Reused row/source revisions cannot publish a prior digest.
Use one narrow source/access query shared with the bounded list rather than repeated stream/item
hydration. Existing grant/device/age/library/label/topology rules remain. Upgrade0069 invalidates
unbound old preparations for explicit release/reprepare and retains their record identity.

Catalog maintenance contention is expected and waits; active hashing keeps restore busy until it
ends/cancels. Root shutdown still joins actual I/O. No source files, external database, transfer API
or client change. Bounded outcome logging remains; no keys, paths, tokens or query data added.

Two regressions failed before the fix: changed source key admitted, and restore crossing a paused
hash. New upgrade/old-backup and maintenance-contention checks accompany existing download tests.
Actual vet/full Go/affected race results and remaining fixture limits are listed in the PR.
Migration numbering and the stacked review base must be reconciled after pending parents merge.

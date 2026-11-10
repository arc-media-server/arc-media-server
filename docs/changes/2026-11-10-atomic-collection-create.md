# #350: create collections and initial members atomically

The real collection API regression reproduced a 400 response after an empty collection had already been saved. It also showed validated sort title, grid mode and locked-field options being dropped. Create the collection and optional manual members in one transaction, validating every member's library/type first. Persist all validated scalar options together. Duplicate member IDs keep their first position.

The existing store method gains optional initial members; existing no-member callers and importer flows keep their interface. Candidate checks use bounded indexed IN batches, not a query per member or filesystem work under the writer transaction. A late membership write failure rolls back the collection as well. Generated identities are published to the caller only after commit.

Tests reproduce failed-create residue/dropped options, then verify the corrected real route, duplicate membership, wrong-library/episode/missing rejection, and an injected membership-write rollback. The importer collection control passes. Its first focused run lacked sandbox loopback permission and failed before executing the fixture; retained separately and rerun successfully with required access. No product failure or provider deployment acceptance is inferred from that restriction.

Validation: API-doc generation, Go vet, full Go suite and API/store race checks passed (API 250.888s; store 237.024s). Full suite includes importer coverage; focused Plex lists control also passed. No API contract/type/generated-doc, web, schema or dependency change. Temporary stores only; no owner media/keys or persistent server.

No new jobs or logs. Expected invalid membership remains ordinary validation; unexpected write failures go through the existing internal-error boundary. Preserve #389's independent wider error-classification/dedup changes when merging the shared collection file. No secrets, raw URLs, request bodies or SQL arguments are logged.

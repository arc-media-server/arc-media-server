# #350: import watch state into the present duplicate

The reported plausible duplicate lookup is reproducible: movies, shows and episodes with the same provider identity selected an older unavailable item even when a present copy existed. Prefer candidates with a present file, then newest added_at and a stable item-id tie-breaker. A series counts as present through its episodes, using the existing shared presence predicate. Keep unavailable matches as a fallback so imports still preserve offline history. Existing provider-id precedence remains.

Episode-number fallback now selects the target across matching shows in one joined query, retaining TheTVDB aired preference before TMDB aired. A show's other present episodes must not make its absent target win over another copy of that target. No identities, media, metadata or existing watch state are changed merely by looking up a match. Imports still apply their existing merge policy.

Regression fixtures reproduce old movie/show/episode choices and episode-number fallback, then verify present/newest and all-unavailable fallback behavior. An early fallback fixture referenced a nonexistent ordering column; corrected to use ReplaceOrdering, with the failed fixture evidence retained separately from the real red regression. All stores and provider fixtures are isolated. No owner keys/media or persistent server; no real-provider or production-scale performance claim. Existing candidate and presence indexes are reused; no schema, API or client change.

No new background work or logs. Existing import outcome diagnostics remain responsible; a successful lookup adds no logging, and no secrets, raw URLs, bodies or SQL arguments are logged.

Validation: reproduced regressions fail against unchanged main and pass with the fix; Go vet, full Go suite and serial store/importer race checks passed (store 328.125s; importer 16.395s). No frontend/API changes or browser checks required.

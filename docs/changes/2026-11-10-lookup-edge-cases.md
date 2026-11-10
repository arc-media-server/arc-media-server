# #350: handle disappearing lookup targets and current provider recovery

Three small API findings reproduced: a library deleted between the item and library reads left seriesFromPath reporting success with a nil library; a title deleted after Fix Match's final successful save returned 500 with a nil internal error; and missing-provider search guidance still pointed to ams.yaml.

Return ordinary 404s for resources that disappear between reads, preserving real database errors as internal failures. Missing-provider guidance now directs the administrator to Settings. No API success shape or new status is introduced.

Tests deterministically delete a library at the existing access-check boundary, and use a trigger in a known temporary database to delete a movie at its final match-trace save. Matching uses a trusted loopback-only TLS provider fixture and fake key. The initial search fixture used query instead of q and failed before reaching the recovery branch; corrected q then reproduced the obsolete message against unchanged code. Keep fixture-error evidence separate from the genuine 500/nil-library/message regressions. A small fixture extraction matches #403's known-path helper; retain it once when merging.

Validation: all genuine regressions failed before the fix, then passed; API-doc generation, Go vet, full Go suite and complete API race tests passed (247.137s). Generated API docs/v1 contract unchanged. No web, media, schema or dependency change, production provider/data or persistent server. Preserve #389's independent provider/error-classification changes in match.go when merging.

Expected disappearance and missing configuration add no server-error logs. Existing request diagnostics remain responsible; no new jobs, keys/tokens, URLs, bodies or SQL arguments are logged.

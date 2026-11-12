# #350: keep file-owned episode identities when provider records change

Real local-provider regressions reproduced both Refresh Metadata and Fix Match replacing canonical
episode ids and losing the visible episode's resume point. Before overwriting source orderings,
snapshot episode keys/file ownership and source positions. After normal file-only/cross-provider
adoption, re-key the prior file-owning episode at that position if the new key is absent and the
old key no longer appears anywhere in the new canonical list. Keep the same Arc item id and all
history/edits/references. Missing files still own their stored row; no media access is needed.

Do not steal a provider identity that merely moved to another position; do not override an existing
new identity or a known cross-provider mapping, adopt ambiguous positions, or reuse one old key twice.
The snapshot uses one parent-indexed query plus the base source entries, not a per-file catalog walk.

Regression coverage uses the real refresh and background Fix Match paths with replaced provider ids,
plus a mixed replacement/reorder where the old id remains elsewhere. Provider lists are local fixtures;
no real keys, owner media or production systems. Empty/shrunken provider-response safeguards are the
next separate #350 change. No API/web/source-media changes. Existing show-match/refresh outcome and
failure logs cover this scope; no new per-episode info logs or URLs/keys/tokens are recorded.

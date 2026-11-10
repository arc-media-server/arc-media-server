# #350: an empty mount point cannot hide its library

A real scan regression replaced a populated scratch root with a readable empty directory: both
titles disappeared, two files became missing and automatic trash was enabled. Snapshot lightweight
known files before discovery, then require a readable known-file anchor in every previously populated
configured root before marking anything missing. Only previously known paths or retained ids seen in this scan qualify; a new
unrelated file or a neighbour on another configured root cannot prove that storage is online. Reuse the
automatic-trash readable-file rule. The seen lookup uses the existing library/last_seen index.

If the proof fails, retain presence state, report an actionable scan failure and do not authorise
trash. Async scan logging emits one warning at its existing outcome boundary, with library and stage;
it is not a server error or a per-file warning flood. No keys, tokens, URLs, bodies or SQL args logged.
Ordinary removals still reconcile when another readable known file proves the folder is online.

A legitimately emptied folder is deliberately indistinguishable from an empty mount point here;
this conservative change does not add a force/confirmation UI. New empty roots with no known files
are allowed. Recognised renames retaining file ids (PR #359) retain their eligibility as anchors.
Physical Unraid/NFS/SMB disconnects remain an owner check; tests use only generated scratch fixtures,
including a partial scan with an available neighbouring configured root. No source-media writes by Arc,
API-field/web changes, owner data or persistent server.

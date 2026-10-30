# Identity-preserving show Unmatch

Added admin POST item/unmatch, explicit matching suppression and filename/file-order restoration.
Metadata, artwork references, edits/locks and provider/custom display orders are cleared atomically;
files, IDs with saved references, watch/resume/history, manual collections/playlists and access labels
survive. File numbering is retained as configuration. Unreferenced provider-only episode caches can
be removed. Explicit Fix Match re-enables metadata; discovery/retry/NFO does not silently rematch.

Shared scoped scanner admission now handles refresh and Unmatch with the existing exclusive library
slot and cancellation policy. Manual Match/Unmatch admission is serialized; background matches are
refused with a retryable busy response. No provider call, media probe, filesystem mutation or UI is
needed for the reset. Full backend/Claude contract: docs/UNMATCH-API.md.

Logging: one bounded successful reset outcome with safe IDs/count/duration; existing failure boundary
handles actionable errors. Expected busy/cancellation/polling adds no error/loop logging. Initial
compile/test fixture mistakes were corrected; final validation: full Go suite passes across 31 tested packages (six have no tests), full vet and focused store/scanner/API race tests pass. A final file-link safeguard was checked again with the Unmatch scanner race tests. Generated OpenAPI/v1 checks and embedded-server build pass; existing production UI build was reused because no frontend source changed. No browser/physical-device or owner-data checks claimed.

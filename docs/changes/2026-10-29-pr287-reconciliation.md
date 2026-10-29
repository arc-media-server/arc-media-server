# Reconcile the v1 contract and logging PR with current main

Resolved PR #287 against main after the owner-list, Android repository notes and teal branding
merges. Recommended keeps its shared category order/visibility/card styles and per-profile
content while retaining the legacy English title, typed rows response and debug timing. Fix
Match retains its background activity ID with the restored 200 success status for series.

Retained main's more detailed Logging and PR review policy and diagnostics checklist, including
expected cancellation, redaction, bounded volume and guards for expensive debug arguments.
Consolidated the overlapping rule and corrected the change note: disabled Go debug calls still
evaluate arguments. No chatty successful-page or polling logs were added.

Regenerated both API references and added 18 contract lines for recent main additions; no existing
contract promise was removed. Added a focused contract test for nested reference/array fields,
renaming, retyping and success-status coverage. Removed an unused library-seeding helper from
the hardware browser spec, which the fresh e2e TypeScript check caught during reconciliation.

Validation: production/translation/app TypeScript build, e2e TypeScript, full go vet ./... and
go test ./..., focused API/scanner race tests, embedded server build and 8/8 browser journeys
passed (Fix Match success/failure/retry, Recommended rendering/customization, Home row editor and
hardware diagnostics). Only temporary stores, generated media and local providers were used.
No physical GPU/HDR, full release-image or owner-server acceptance is claimed. Hosted workflows
are not waited on. Contract limits in the original v1 change note remain: request/query inputs
and untyped free-form response fields are not comprehensively covered.

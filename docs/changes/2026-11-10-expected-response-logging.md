# Distinguish expected unavailability from server failures

A missing subtitle API key returned the correct 503 but logged an Arc request
failure every time. Known optional-service/configuration absence and conversion
capacity limits now explicitly mark the response as expected. They remain visible
with status/duration in detailed request logs. Unknown 502/503 responses and real
database/internal failures still produce diagnostics; there is no blanket 5xx
suppression.

OpenSubtitles search/download failures have one safe provider/operation/request-ID
warning, rather than a generic Arc failure summary. No upstream error text or raw
URL/key is added to the log. Existing response statuses/bodies are unchanged.
Central request cancellation is quiet, and already-owned 5xx failures no longer
produce a duplicate slow-response warning.

Red regressions reproduced the missing-key warning, cancelled-request error and
missing provider classification. Positive controls retain warnings for unknown
502/503 and one error for unexpected internal failures. A non-admin fixture first
returned the correct 403; the corrected owner-role fixture established the actual
503 regression. Local provider fixtures and fake keys only.

The existing #357 failure-dedup foundation is retained. Preserve #389's other
provider/internal classifications and #410's shared subtitle extraction diagnostic
owner on integration. No success-request info flood, raw bodies, keys, URLs,
SQL arguments, API shape/schema, dependencies, frontend or media changes.
Part of #350 API Low 12.

Validation: go run ./tools/apidoc, go vet ./..., go test ./... and API race
passed (249.949s). Generated API/schema/v1 contract files are unchanged.

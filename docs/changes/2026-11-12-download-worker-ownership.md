# Common shutdown ownership for download preparation

Quality review CQR05: register the download preparation loop with the existing named process worker group. Its cancellation and join now precede the common stopped outcome and store closure, with the same bounded slow-shutdown diagnostic as other workers. Remove the second private context/channel/join. Source checks, grants, catalog reservations and durable queued work remain.

The contributor map now names lifecycle, downloads and filecache, linked from README Development. No new framework, goroutine policy or API. A blocked OS file read can still delay safe shutdown; this change makes ownership visible and continues waiting rather than closing its dependencies early.

Validation results are listed in the PR. Temporary fixtures only. Logging reuses the single threshold-based shutdown warning with a static operation name; no item paths, request data or secrets added.

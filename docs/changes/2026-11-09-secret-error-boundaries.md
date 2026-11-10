# #350: keep provider keys and webhook tokens out of errors

Failing fake-transport regressions reproduced TMDB/OMDb keys in returned errors, the TMDB match
trace, and a webhook token in both its delivery result and warning log. Sanitize credential-bearing
request construction/transport failures at those boundaries. Classify failed/cancelled/timed-out
requests without rendering url.Error, wrapped error text or redirect URLs. Preserve errors.Is for
cancellation and other causes, without exposing the original URL wrapper through errors.As.
HTTP status errors remain useful; no request location or response body is logged.

Scrub URL/credential text in match-explanation error fields on write and read, including restored
legacy rows. Keep artwork URLs, other fields and integer precision. Migration 0064 clears historic
explanations containing API-key/access-token markers, so ordinary future backups no longer copy
them. Only derived explanations are cleared; metadata, matches, files and history are unchanged.

Existing backups and the automatic pre-upgrade recovery backup may still contain the historical
text; this change does not rewrite/delete backups or access the owner's data. Rotate exposed keys
and handle old backup retention as an owner operation if exposure occurred. No production exposure
is claimed from these fake-credential tests. Webhook retry behaviour remains a separate #350 item.

Logging retains one existing failed-delivery warning with safe operation/event context; transport
failures now use bounded categorical text. No additional success/per-item logs or raw URLs/keys.

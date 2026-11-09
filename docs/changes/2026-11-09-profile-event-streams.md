# #350: reserve event-stream capacity per profile

The server-wide 32-stream limit let one signed-in profile consume every slot. A real handler
regression opened eight streams then confirmed the ninth was accepted before the fix.
Limit a profile to eight concurrent streams, leaving capacity for other profiles and multiple
legitimate devices/tabs per profile. Preserve the global 32-stream cap and existing retryable
429 response. Authentication is checked before slot reservation. Hub counters are protected by
one mutex; all existing exit paths release both counters and empty profile entries are removed.

Tests cover the ninth refusal, another profile's successful stream, released-slot reconnects,
concurrent admission, global capacity, replay/revocation and unpaired-close safety. Only isolated
stores and synthetic sessions were used. API shape is unchanged; the handoff documents shared
profile capacity and Retry-After. No new logs, per-event work or goroutines; ordinary admission
limits are expected validation rather than server failures. No credentials, bodies, raw URLs
or SQL arguments are logged. A profile with many legitimate tabs may need to close some; the
existing automatic client reconnect/backoff still applies. This is not a per-IP/account quota.

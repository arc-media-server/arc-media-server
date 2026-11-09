# #350: keep password work for unknown friend names

Friend sign-in returned immediately for unknown names and pending invitations, whereas an
existing password ran bcrypt. The real handler regression reproduced that missing work first.
Select a public dummy hash at the same default cost when no stored credential exists, always
compare, and independently require an existing friend with a non-empty password before success.
The dummy's known input is explicitly tested to stay unauthorized and set no session cookie.

Keep the generic failure response and existing limiter/outcome logs; no names, credentials or
hashes are added to logging. No new logs or goroutines. Failed admitted unknown-name attempts
now intentionally pay the same password-hashing work; #354's atomic limiter must be retained
when merging. This removes the large bcrypt timing difference, not a claim of constant-time SQL,
network responses or malformed database hashes. The timing regression has an 8x margin against
the fastest of three real default-cost comparisons; it is not a performance benchmark.

Only temporary stores and synthetic profiles were tested. No web/API shape changes.

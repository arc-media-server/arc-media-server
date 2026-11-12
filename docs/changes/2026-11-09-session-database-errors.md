# #350: do not turn session database failures into sign-outs

A failing regression with valid cookie and Bearer credentials plus unavailable storage returned
401, encouraging apps to discard valid tokens. Session lookup now returns its error separately
from an absent/revoked session. HTTP auth/profile callers return 503 and Retry-After: 5 on lookup
failure, without clearing the cookie/token. Missing, revoked and no-longer-home local sessions
retain 401. Watch Together propagates the storage failure instead of treating it as changed auth.

Event authorization also distinguishes storage failures: initial lookup returns a server error;
a stream already open closes on a failed revalidation, logging once without claiming revocation.
No success response fields or token semantics change. Regenerate API documentation.

Session failures log once with request id/operation and database error, never the presented token.
Expected request cancellation avoids a server error log. An error logged at its responsible boundary
is not repeated by the request summary (also applies to existing fail boundaries). Routine success
logging is unchanged. Tests cover cookie/Bearer 503, Retry-After, no cookie clearing, one diagnostic
without token disclosure, normal 401 and event-principal error propagation. Real TV/device/network
recovery remains an owner check; no production data used.

# Import job lifecycle: client handoff (#350)

Successful JSON envelopes and stage names are unchanged for `/api/v1/imports` and the web aliases.

| Action | Behavior |
| --- | --- |
| POST `/imports/{id}/run` | Only one request may prepare or run this session. Concurrent/incorrect-stage calls return 409. Disable Run while its request is pending; refetch the session on 409. |
| Map accounts to profiles | Unknown account ids, missing targets or no selections return 400 before profile creation. New profiles are created together; a generated-name collision returns 409 and leaves none behind. Choose existing profiles to resolve a collision. |
| Catalog maintenance | Run returns 503 with Retry-After: 3 before creating profiles. An active import prevents destructive catalog maintenance through the shared reservation gate. |
| Server stopping | Admission returns 503. Retry after restarting/reconnecting, rather than starting duplicate requests. |
| POST `/imports/{id}/server` | Returns 409 if this session is preparing/running or in another incompatible stage. |
| GET `/imports/{id}` | Running/preparing sessions stay available past one hour. Finished results stay for one hour after completion; expired sessions return 404. |
| Server/listener lifecycle | Changing the listener leaves accepted jobs running. Server shutdown cancels and joins this job. Memory sessions do not survive process restart; start a new import. Existing imported watch state is merged, not erased. |
| Failure text | Top-level job/activity errors give a safe recovery message. Render as text. Start a new import after a failed/interrupted session; this is not automatic durable resumption. |

No new frontend is required to consume the unchanged success responses. Preserve the current wizard, pending state and session polling. Backend import ownership does not establish successful production-provider migration or full-server shutdown acceptance.

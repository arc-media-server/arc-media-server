# #350: reserve sign-in attempts before checking credentials

A failing real-handler regression admitted all 20 simultaneous wrong password/PIN guesses,
because Allow and Fail were separate operations. Use the existing atomic Limiter.Attempt at
household login, PIN/admin-profile selection, password changes, friend login, invite redemption
and device-code approval. Remove the later Fail calls so a wrong attempt counts exactly once.
Successful authentication retains the existing Succeed reset. A database/validation failure after
admission also consumes that bounded slot; a request cannot avoid admission by failing later.

The concurrent regression covers all seven paths and checks that at most five reach verification.
Ordinary logging coverage is unchanged: failures and admission denials retain their existing safe
context, with no new logs or credentials/body/URL logging. No browser, response-shape, setting or
media changes. The anonymous device-link start pool is addressed separately in #350.

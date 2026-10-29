## Change

Describe the user problem and resulting behaviour.

## Validation

List actual checks and material limits. Do not claim production/device acceptance from fixtures.

## Logging and diagnostics

- [ ] Failures and stalls are diagnosable, or explain why no additional logging is needed.
- [ ] Meaningful outcomes/decisions have appropriate context; expected cancellation/empty results
      are not errors, and failures are not duplicated across layers.
- [ ] Detail is debug-only; hot paths avoid info floods and expensive disabled-log preparation.
      Repeated warnings/diagnostic collection are bounded.
- [ ] No secrets, bodies, raw URLs/query strings or SQL arguments are recorded.

What is logged, when, and what are the volume/performance bounds?

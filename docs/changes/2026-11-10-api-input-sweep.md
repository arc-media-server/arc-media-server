# Keep real-server malformed-input coverage

Add an opt-in Linux integration test driven by the generated OpenAPI operations.
It race-builds the actual server, starts it with a disposable database, creates a
fixture owner, and checks malformed JSON, invalid query values and nonexistent
path identifiers through both /api and /api/v1. It uses normal server dependencies,
not nil-handler stand-ins. Provider URLs point only at a loopback failure fixture;
production credentials/settings/media are never used.

The test checks transport/panic/internal-error failures and top-level null arrays
where the API specifies an array. Nested null sentinels such as profile.library_ids
are retained. Invalid authentication inputs may legitimately answer 401; a final
protected request verifies the fixture session remains valid. Unavailable fixture
features/providers may answer 502/503/504 without counting as feature acceptance.

Run explicitly from the prepared environment:
ARC_RUN_API_INPUT_SWEEP=1 go test -race ./cmd/ams -run '^TestAPIInvalidInputSweep$' -count=1 -v

Actual final result: 406 input checks passed, zero failed, seven operations skipped
(14 alias requests): logout, profile leave, create backup, news seen, remote check,
hardware detection and the live events stream have no malformed body/path input
to exercise here. The 406 checks include two expected 503 unavailability responses;
no provider calls were needed. Positive execution of skipped commands is outside
this input sweep. Other platforms and physical devices are not validated.

The temporary server stops at teardown and uses a Linux parent-death signal as a
backstop if the test is interrupted. Vet/full Go passed; the opt-in race test
passed in 7.539s (server race-built too). No product/API/schema/dependency or media
change, no new application logging, and no persistent Arc server. Part of #350's
permanent route robustness coverage.

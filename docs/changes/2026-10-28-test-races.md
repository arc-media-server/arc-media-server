# Two test fixes found by Codex's integration run (28 October)

- **Throttle test race.** `TestConversionWaitsForThePlayer` changed package-level timing values
  while another session's throttle goroutine could read them (`go test -race` flagged it about
  1 run in 11). The thresholds are now a `throttling` value on the Manager (default 2 minutes /
  1 minute / 1 s), copied into each session when it starts; the test configures only its own
  manager. Production behaviour is unchanged.
- **Cast subtitle journey and LAN addresses.** Arc hands a cast device its address on the home
  network, so the journey's direct fetch of the WebVTT link failed where the fixture server only
  listens on 127.0.0.1. It now checks the link's shape and fetches the same path on the page's
  own host. Production behaviour is unchanged.

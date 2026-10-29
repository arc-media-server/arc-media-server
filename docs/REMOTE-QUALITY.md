# Remote quality and speed measurement

Arc already keeps compatible sources at original quality when they fit the effective remote
quality limit. The player must support the source container, codecs and selected subtitles;
a low bitrate alone cannot guarantee Direct Play. A container change can use Direct Stream
(remux), while incompatible codecs or burned subtitles require conversion.

The effective ceiling considers the server's per-stream limit, upload budget and the profile's
Internet streaming quality. Quality presets also set a resolution ceiling. The current quality
comparison allows 10% tolerance for reported bitrate/height. Unknown source bitrate is treated
conservatively when a limit applies. Inspect Dashboard for the actual session method and playback
decision logs for conversion reasons. Source HDR/Atmos badges describe the file, not output.

Set Internet upload speed from a measurement on the server's own connection (preferably from
that machine, with other traffic quiet). Remote streams share 80% of the configured upload
capacity. A speed test run on a remote phone measures that phone's connection, not the server's
upload. A phone downloading from Arc measures that particular end-to-end route, which may be
limited by either connection, Wi-Fi or the route itself.

An automatic WAN test requires an external endpoint able to receive and measure uploads.
Arc does not run a speed-test service or silently send test traffic to a third party. Keep the
manual measured value for now; adding an explicit test needs a provider, disclosure, bounded
traffic/timeouts and a distinction between server upload capacity and a client's route.

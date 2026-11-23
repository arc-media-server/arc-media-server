# Sending device logs

Arc apps send logs only when someone chooses **Send logs** or accepts a crash report prompt. Upload to the current Arc server over the app's normal authenticated connection. The logs never go to a third party. The server admin reads or downloads them under **Dashboard → Devices** in the log section.

## Uploading

Send `POST /api/v1/client-logs` with the signed-in app's Bearer token and a selected profile. A successful upload returns `204`. Browser and tool sessions cannot upload. Older servers return `404`, so the client should explain that the server needs an update. Do not retry automatically.

```json
{
  "platform": "roku",
  "entries": [
    { "time_ms": 1795420800000, "level": "error", "event": "playback_failed" }
  ]
}
```

- Send 1–3,000 entries in chronological order, at most 256 KiB per request. Keep only the newest entries if necessary.
- Platform is `android`, `apple`, `kodi`, `roku` or `tv`. Level is `debug`, `info`, `warn` or `error`. Timestamps are Unix milliseconds.
- Use only the fixed events below. Project local events onto these names before storing or sending them. Drop unrecognised lines. Never attach their original text, exception messages or stacks, IDs, server addresses, URL paths, requests, headers, tokens, PINs or passwords.
- Keep debug events only while temporary detailed logging is active. Use `crash_detected` for a known crash on platforms that can report one. Offer to send it on the next launch and upload only if accepted.

| Area | Events |
| --- | --- |
| App and sign-in | `app_started`, `profile_picker`, `automatic_sign_in`, `sign_in_failed`, `session_expired` |
| Network and library | `request_failed`, `request_timeout`, `library_load_failed`, `recommendations_load_failed` |
| Playback | `playback_requested`, `playback_started`, `playback_paused`, `playback_resumed`, `playback_ended`, `playback_failed`, `playback_buffering`, `playback_seek_failed`, `subtitles_failed` |
| Diagnostics and downloads | `detailed_logging_enabled`, `detailed_logging_expired`, `crash_detected`, `download_failed` |

## Retention

The most recent four uploads per signed-in device are kept for seven days after receipt, up to 128 uploads for the whole server. An expired upload is hidden immediately and removed by daily cleanup or the next upload. Revoking a device also removes its live logs. Existing database backups can contain snapshots of uploaded events until those backups expire. Server diagnostics downloads do not include these events.

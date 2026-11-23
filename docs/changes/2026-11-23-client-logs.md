# Device logs in the Dashboard

Arc admins can read and download app logs from Dashboard → Devices. The separate app changes add
the manual Send logs action. The existing server log view remains the default.

- Only signed-in app sessions can upload. Arc accepts fixed event codes, levels and timestamps.
  It rejects arbitrary log text and unknown fields. The selected device's name comes from the
  existing session and is excluded from the downloaded file.
- Four uploads per device, 128 uploads for the server, at most 256 KiB per upload. Entries expire
  seven days after receipt. Device revocation removes its live uploads. Database backups may keep
  previously included batches until the backups expire.
- Successful uploads log a debug line with the platform and event count. Expiry cleanup logs only a
  nonempty debug outcome or a warning if it fails. No diagnostic event bodies appear in server logs.

Go vet, API generation, production web build, TypeScript and the scratch Dashboard/download browser
journey passed. The full Go run surfaced two migration test fixtures that needed to drop the new
table when reconstructing an older schema. The store suite passed after updating those fixtures.
Native devices and deployed servers were outside this check.

[Device upload and retention contract](../CLIENT-LOGS.md)

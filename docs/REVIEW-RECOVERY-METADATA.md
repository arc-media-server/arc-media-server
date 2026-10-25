# Recovery, local metadata and retained decisions (#136/#191/#194/#104/#195)

Review baseline: 7659488, 25 October 2026. These five items come from audit findings
F14/F23/F30/F31/F32 in #162. A finding is reviewed individually; optional Plex features
are not automatically accepted changes.

| Finding / issue | Decision | Work and remaining boundary |
| --- | --- | --- |
| F14 / #136 | Implement clearer backup scope and rehearse full-folder recovery; keep readiness issue open | Backups and its confirmation distinguish SQLite from files; RECOVERY.md provides the stopped-copy/rollback procedure; isolated upload/DB recovery evidence added. Owner storage, power-loss and external-config acceptance remain |
| F23 / #191 | Correct documentation; broader NFO import not accepted | README and LOCAL-METADATA.md explain actual identification/artwork scope, ignored text and episode sidecars. No new importer or precedence setting |
| F30 / #194 | Retain approved profile language learning | Player/preplay learn deliberate audio and non-forced subtitle choices; Settings explains audio learning. Existing default/forced/Off and successful-preplay-admission rules remain |
| F31 / #104 | Close the older backfill proposal as superseded | Series details use the selected details provider; episode text/stills/runtime use source numbering provider. Mapping is not field backfill. Blank-field and switch/identity regression assertions added |
| F32 / #195 | Retain simpler choices; no product change | Global scan scheduling, shared edition watch identity, local accounts/personal-media focus and no source-media deletion remain. Separate later/native/public-release issues stay separate |

## Why these decisions

The owner's [single-provider decision](https://github.com/JonzaUK/arc-media-server/issues/80#issuecomment-6251381329)
explicitly keeps missing provider fields blank and ties episode text to numbering. This
supersedes #104, not episode identity or manual locks. Changing it as an automatic gap-fill
would reverse that decision. LOCAL-METADATA.md records both scopes and the available
Details From/manual-edit choices; source numbering must match the files, not be changed
casually to obtain one description.

The owner's [language-learning decision](https://github.com/JonzaUK/arc-media-server/issues/80#issuecomment-6241883327)
and [approved UI defaults](https://github.com/JonzaUK/arc-media-server/issues/80#issuecomment-6341453397)
remain current. Player selectSubtitle and audio menu update profile preferences for manual
choices; preplay applies learning only after successful start. Non-forced subtitle language
can be learned, forced choices preserve it, and Off clears the old remembered-language
mode only where appropriate. Settings offers an explicit profile audio preference too.
No extra “make default” setting is added without a new accepted task.

For F32, [editions share identity](https://github.com/JonzaUK/arc-media-server/issues/80#issuecomment-6233582700)
by design. The [owner-selected global scan controls](https://github.com/JonzaUK/arc-media-server/issues/80#issuecomment-6340025778)
remain server-wide, with ordinary manual library actions separate. Empty Trash/Forget remove
library database records, not source media. Local account setup does not depend on Plex's
account service; optional Plex import is a separate user-invoked feature. Music, other media,
native apps and public-distribution issues are not automatically new movie/TV defects.

## Evidence and limits

Code references: internal/backup/backup.go and store/backup.go; api/artwork.go and image
serving; scanner/local.go, scanner.go and shows.go; Player.tsx, lib/playback.ts and Settings.tsx;
store/health.go, api/auth.go and docs/SCANNING-CONTROLS.md. Exact completed commit and fresh
results are in READINESS and the GitHub issues/#162, so historical audit evidence is not
substituted for current verification.

The current [Plex NFO guide](https://support.plex.tv/articles/using-nfo-metadata-files-with-plex/)
was readable and confirms a broader 1.43.1+ movie/series NFO agent, with its own limitations.
The official Plex backup-restore and account-language pages returned 403 in this review;
exact current behavior for those pages is unverified. Retaining Arc's approved policies does
not depend on an assertion about Plex internals or exact parity.

Only temporary stores, generated clips/images and local providers are used. RECOVERY.md
separates a stopped-store/API folder rehearsal and existing container acceptance from real
NAS, external TLS, full-filesystem or abrupt whole-server acceptance. Certificate fixture
bytes test copying, not real TLS. No owner data/settings/media/keys or Unraid access and
no tool/dependency installation or persistent server.

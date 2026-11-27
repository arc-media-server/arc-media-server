# Choose how Arc labels seasons

Show libraries default to Season 1, Season 2 and so on. The library's Advanced settings can
use names from the episode order when available. A show can follow its library or choose its
own setting in Edit → Metadata & ordering. Specials and manually locked names take priority.

The setting affects the season label returned by the server. Episode order, file links and
watch state use the same existing identities and positions. Provider season names stay stored,
and the Seasons editor can still show them. No metadata refresh or library scan is needed.

Existing SQLite and PostgreSQL stores pick up the number default on upgrade. The API adds
`season_label_mode` and supplies `metadata_name` when the stored name differs from the label.
There are no additional log entries. The existing request logs cover saves and errors.

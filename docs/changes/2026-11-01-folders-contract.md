# Folder browsing's answer is part of the v1 promise

**What changed.** `GET /libraries/{id}/folders` (an Other videos library folder by folder) now
answers with a named type (`folderPage`: path, trail, folders, items) instead of a loose map. The
JSON is identical; the difference is that the API docs and `docs/api-v1-contract.txt` now list
its fields, so the TV apps (arc-tv, arc-android), which add a Folders view for home videos, can
rely on them and a test fails if one changes.

**Why.** The TV apps use only what the contract promises; this endpoint's status was listed but
none of its fields were.

**Logging.** Nothing new: the handler is unchanged apart from its answer's type.

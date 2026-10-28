# Folder view for Other videos libraries (#80 item 28, 28 October), server side

`GET /api/libraries/{id}/folders?path=` browses an Other videos library folder by folder, as
Plex can: the folders inside `path` (each with how many videos it and its subfolders hold, and
its oldest video as a cover) and the videos directly in it, oldest first (by the date in the file
name, else the file's date: the same order as folder collections), plus a trail back up. A
library with one folder starts inside it; with several, the top lists each by name. Paths are
`<library folder number>/<folder>/<folder>`. It follows the profile's library access, age limit
and hidden labels, and only lists videos with a file present. Other libraries get 400.

Unlike folder collections (one per folder that directly holds videos, kept as they are), this
shows nesting and folders that only hold other folders.

UI: a Folders tab next to Videos and Collections on the library page follows once the open
translation PR for the Library page (#238) merges, as agreed on #80.

Checks: go vet/test (new API test: top level, nested counts and covers, oldest-first order,
trail, unknown folder 404, non-video library 400).

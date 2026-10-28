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

In the app: Other videos libraries get a **Folders** tab between Videos and Collections. It shows
the current folder's subfolders as cards (cover picture, "Folder" tag, video count), then its
videos, with a trail back up. The folder is in the address (`?folder=`), so Back, refresh and links
work; switching tabs leaves the folder. Movie and TV libraries have no Folders tab.

Checks: go vet/test (new API test: top level, nested counts and covers, oldest-first order,
trail, unknown folder 404, non-video library 400). Playwright: a journey scans a real nested Home Videos folder, opens
folders down to 2019, checks counts, the trail and oldest-first order, goes Back, returns to the
top by the trail, and checks a movie library has no Folders tab.

# Artwork and collection automation

Arc already accepts poster uploads and manual collection updates through its API. An adapter
for Arc is needed: this does **not** make Plex-oriented tools such as Kometa or PosterFlow
compatible without changes. Use the versioned `/api/v1` routes in external tools; `/api` is
for the bundled web app. See [API reference](API.md) for the complete contract.

## Authenticate once, then select an administrator profile

Use your server's HTTPS address for remote connections. Send JSON with `Content-Type:
application/json`. Do not put passwords or tokens in URLs, checked-in scripts or debug output.

1. `POST /api/v1/auth/login` with `{"password":"…","device":"My artwork automation"}`.
   Keep the returned `token` privately and send `Authorization: Bearer <token>` on later requests.
2. `GET /api/v1/auth/status` lists profiles. Select an administrator with
   `POST /api/v1/auth/profile`, body `{"id":"<profile-id>","pin":"…"}` (PIN if required).
3. These are device/session tokens, not scoped automation API keys. An administrator token has
   administrator permissions. Revoke it through Settings > Access & devices when no longer needed.

Alternatively, use the existing device-link flow: `POST /api/v1/auth/link` with a device name,
show its `code`, and approve it through Arc's profile menu > Link a Device. Poll the returned
`poll` identifier with `GET /api/v1/auth/link/{poll}` every few seconds: 202 means pending,
200 returns a token once, and 404 means it expired. Codes expire after ten minutes. Do not
persist the displayed code as a credential.

## Find the target

`GET /api/v1/libraries` returns permitted libraries. Query
`GET /api/v1/items?library_id=<number>&kind=movie&limit=100&offset=0` (use `series` for shows).
Read the `items` and `total` fields and continue with `offset=100`, etc. Keep the returned Arc
item IDs and use provider IDs to cross-reference titles when supplied; a title alone is not a
unique identifier. Do not guess an ID from a filename or an external provider's numeric ID.

## Upload a poster

`POST /api/v1/items/{id}/artwork/poster` accepts the **raw image bytes**, not multipart form data
or a JSON URL. Supported content types are `image/jpeg`, `image/png` and `image/webp`; maximum
upload size is 20 MiB. `backdrop` and `logo` use the same route shape; `primary` aliases `poster`.
Arc stores the upload in its own data directory, never alongside media, and locks that artwork
choice so metadata refreshes preserve it. A successful upload returns the updated item.

Example request, with credentials supplied securely by your HTTP client:

```http
POST /api/v1/items/<arc-item-id>/artwork/poster
Authorization: Bearer <private-session-token>
Content-Type: image/jpeg

<raw JPEG bytes>
```

`GET /api/v1/items/{id}/artwork` lists provider choices. `PUT /api/v1/items/{id}/artwork`
with `{"type":"poster","path":null}` unlocks the choice and restores automatic artwork.
For provider selection, use a returned provider path, not an arbitrary image URL; upload your
own image instead. Local artwork precedence can still apply: inspect the artwork response's
`local`, `current` and `chosen` fields when diagnosing what is displayed.

## Create or fill a manual collection

```http
POST /api/v1/collections
Authorization: Bearer <private-session-token>
Content-Type: application/json

{"library_id":1,"kind":"manual","title":"Friday night","item_ids":["<arc-movie-id>"]}
```

The response contains the collection ID. Add titles with
`POST /api/v1/collections/{id}/items`, body `{"item_ids":["<arc-item-id>"]}` (204 on success).
Titles must be movies or series from the same library; episodes are not collection members.
Manual membership routes do not edit provider-managed, smart or folder collections. List
existing collections with `GET /api/v1/libraries/{id}/collections` before creating one to avoid
duplicate names. Remove membership with `DELETE /api/v1/collections/{id}/items/{item}`.
Collection artwork also supports `POST /api/v1/collections/{id}/artwork/poster` with raw bytes.

## Adapter behaviour

- Check HTTP status and the JSON `error` field. A 401/403 requires authentication/permission
  correction; do not repeatedly retry it. Validate images locally before uploading.
- Fetch IDs afresh after a database restore or title merge. Do not recreate a missing item.
- Retry temporary server/network failures with bounded backoff. After an ambiguous collection
  creation response, list collections before retrying to avoid duplicates.
- Reuse existing collections and membership. Compare desired artwork/content before uploading
  repeatedly. Avoid triggering metadata refresh or analysis after every upload.

These routes were checked against the implementation and existing API tests. No third-party
adapter, owner's server or real external automation application was exercised in this change.

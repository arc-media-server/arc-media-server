# Local metadata and artwork

Arc reads local assets without writing beside or changing media. Its NFO support is
an identification aid, not a complete offline metadata agent.

## What a scan reads

| Input | Current behavior |
| --- | --- |
| Movie `<video filename without extension>.nfo` | Read provider IDs; takes precedence over folder-wide movie.nfo if it has usable IDs |
| `movie.nfo` | Read IDs only when the movie has its own recognized folder; ignored as a folder-wide source in a shared movies directory |
| Show-root `tvshow.nfo` | Read TMDB, TVDB and IMDb IDs before series matching |
| `<uniqueid type="tmdb/tvdb/imdb">` | Parse the corresponding ID; first recognized value for each provider wins |
| `<tmdbid>`, `<tvdbid>`, `<imdbid>`, IMDb `<id>` or IMDb URL/text | Supported identification fallbacks; movie scanning adopts TMDB/IMDb IDs, show scanning also adopts TVDB IDs |
| NFO `<title>`, `<plot>`, genres, cast, ratings or artwork URLs | Not imported as displayed metadata |
| Episode or season NFO sidecars | Not consumed by scanning. The low-level ID parser can read an episodedetails XML root, which does not mean the scanner imports episode sidecars |
| Missing/malformed/text-only NFO without recognized IDs | No NFO identity; normal matching/unmatched behavior continues. The parser can still recover a recognizable IMDb ID from raw text |

The NFO reader is bounded to 1 MiB. ID-only files do not supply an offline synopsis or
cast: metadata still comes from the configured provider, or deliberate edits in Arc.
A title/plot-only NFO does not create that local metadata. Existing manual field locks
remain authoritative during provider refresh.

Local poster/background/logo and episode-thumb files remain supported independently
of NFO. Movie filename-specific artwork works in shared directories; folder-wide images
are used for recognized own-folder movies. Shows can use poster/folder/cover/show,
fanart/backdrop/background/art, clearlogo/logo and season posters. Supported image
extensions are jpg/jpeg/png/webp; exact naming is in scanner/local.go. Local theme tunes
are independent too. Uploaded artwork is kept in Arc's data folder, not beside media.

## Provider scope (#104)

Series-level details/artwork use TMDB by default, an explicit **Details From** choice,
or the existing fallback when a show is absent from one provider. A missing field on
an otherwise matched provider is left blank, not silently filled from another source.
Episode titles, descriptions, stills and runtime follow the provider used for source
numbering, independently of series Details From and display ordering. Alternate-provider
records are used to map identities/orders, not to mix missing episode fields.
Manual edits/locks and watch identity survive supported provider switches.

This retains the owner's recorded single-provider decision in
[#80 comment 6251381329](https://github.com/JonzaUK/arc-media-server/issues/80#issuecomment-6251381329).
The older #104 cross-provider backfill proposal is superseded by that policy, not an
unimplemented repair to be enabled automatically.

## Comparison and possible later work

[Plex's current NFO documentation](https://support.plex.tv/articles/using-nfo-metadata-files-with-plex/)
describes a broader movie/series agent in server 1.43.1+, including text and cast fields.
It has its own limitations; this does not establish Arc parity or require copying it.
A broader Arc importer would need an actual collection need, supported fields, precedence,
provenance, language and lock behavior agreed first. No full NFO importer is introduced
by the #191 documentation correction. Validation and limits are in READINESS/#191.

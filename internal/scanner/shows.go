package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"ams/internal/metadata"
	"ams/internal/store"
)

// Series metadata (and their orderings) are refreshed at most this often during scans.
const resyncAfter = 7 * 24 * time.Hour

type episodeFile struct {
	fileID int64
	path   string
	ep     EpisodeName
}

type seriesGroup struct {
	name MovieName
	// The series' own folder, or "" when its files sit in the library folder itself.
	folder string
	files  []episodeFile
}

func (s *Scanner) collectEpisodeFile(ctx context.Context, lib *store.Library, root, path string, d fs.DirEntry,
	scanStart int64, groups map[string]*seriesGroup) error {
	fileID, err := s.recordFile(ctx, lib, path, d, scanStart)
	if err != nil {
		return err
	}
	key, name, ep, ok := ParseEpisodePath(root, path)
	if !ok {
		s.update(lib.ID, func(st *Status) { st.Unrecognised++ })
		s.log.Info("not recognised as an episode", "path", path)
		return s.store.SetFileItems(ctx, fileID, nil, 0)
	}
	g := groups[key]
	if g == nil {
		g = &seriesGroup{name: name}
		groups[key] = g
	}
	if g.folder == "" {
		if rel, err := filepath.Rel(root, path); err == nil && strings.Contains(filepath.ToSlash(rel), "/") {
			g.folder = filepath.Join(root, strings.Split(filepath.ToSlash(rel), "/")[0])
		}
	}
	g.files = append(g.files, episodeFile{fileID: fileID, path: path, ep: ep})
	return nil
}

func (s *Scanner) syncSeriesGroups(ctx context.Context, lib *store.Library, groups map[string]*seriesGroup, log *slog.Logger) error {
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := s.syncSeriesGroup(ctx, lib, groups[k], log); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Warn("series failed", "series", groups[k].name.Title, "err", err)
		}
	}
	return nil
}

func (s *Scanner) hasProviders() bool { return s.tvdb.Configured() || s.tmdb.Configured() }

func (s *Scanner) syncSeriesGroup(ctx context.Context, lib *store.Library, g *seriesGroup, log *slog.Logger) error {
	id, created, err := s.store.EnsureItem(ctx, &store.Item{
		LibraryID: lib.ID, Kind: "series", Title: g.name.Title, Year: g.name.Year,
		TVDBID: g.name.TVDBID, TMDBID: g.name.TMDBID, IMDBID: g.name.IMDBID,
	}, seriesMatchKey(g.name))
	if err != nil {
		return err
	}
	if created {
		s.update(lib.ID, func(st *Status) { st.NewItems++ })
	}
	// Local artwork and tvshow.nfo ids from the show's folder (before looking it up, so the ids are used).
	if g.folder != "" {
		art := seriesArt(g.folder)
		if err := s.store.SetLocalArt(ctx, id, art.Poster, art.Backdrop, art.Logo); err != nil {
			return err
		}
		if nfo := readNFO(filepath.Join(g.folder, "tvshow.nfo")); !nfo.empty() {
			if err := s.store.AdoptIDs(ctx, id, nfo.TMDBID, nfo.TVDBID, nfo.IMDBID); err != nil {
				return err
			}
		}
	}
	syncedAt, err := s.store.SyncedAt(ctx, id)
	if err != nil {
		return err
	}
	age := time.Since(time.Unix(syncedAt, 0))

	synced := false
	if s.hasProviders() && (syncedAt == 0 || age > resyncAfter) {
		if err := s.SyncSeriesMetadata(ctx, lib, id, false); err != nil {
			if ctx.Err() != nil {
				return err
			}
			log.Warn("series metadata failed", "series", g.name.Title, "err", err)
		} else {
			synced = true
		}
	}

	// Files naming episodes the cached metadata doesn't know yet are usually new episodes:
	// refresh once (at most hourly) before treating them as unknown.
	canResync := !synced && s.hasProviders() && syncedAt != 0 && age > time.Hour
	unknown, err := s.linkEpisodeFiles(ctx, lib, id, g.files, !canResync)
	if err != nil {
		return err
	}
	if unknown > 0 && canResync {
		log.Info("unknown episodes found, refreshing metadata", "series", g.name.Title, "files", unknown)
		if err := s.SyncSeriesMetadata(ctx, lib, id, false); err != nil {
			log.Warn("series metadata failed", "series", g.name.Title, "err", err)
		}
		if _, err = s.linkEpisodeFiles(ctx, lib, id, g.files, true); err != nil {
			return err
		}
	}
	// Episodes left behind by an earlier, wrong match no longer have files: clear them out.
	if n, err := s.store.DeleteOrphanEpisodes(ctx, id); err != nil {
		return err
	} else if n > 0 {
		log.Info("removed episodes no longer in the series", "series", g.name.Title, "count", n)
	}
	return nil
}

// SyncSeriesMetadata fetches a series from TVDB and TMDB, creates its episodes and rebuilds every
// provider ordering. Episodes are identified by the provider whose numbering the files use
// ("canonical"); the other provider's episodes are mapped onto them.
//
// Mapping by remembered ids and by air date + title is free and always done. The thorough mapping
// (a TMDB lookup per remaining episode, then looser matches) only runs when deep is set or the
// series is displayed in the other provider's ordering, since that's the only time it matters.
func (s *Scanner) SyncSeriesMetadata(ctx context.Context, lib *store.Library, seriesID string, deep bool) error {
	series, err := s.store.GetItem(ctx, seriesID)
	if err != nil {
		return err
	}
	if series == nil {
		return fmt.Errorf("series %s not found", seriesID)
	}
	log := s.log.With("series", series.Title)
	ctx, save := s.traced(ctx, series.ID)
	defer save()

	var tvdbSeries, tmdbSeries *metadata.Series
	var orderings []metadata.Ordering
	tvdbID, tmdbID := series.TVDBID, series.TMDBID

	fetchTVDB := func() error {
		ser, types, err := s.tvdb.Series(ctx, tvdbID)
		if err != nil {
			return fmt.Errorf("tvdb series %d: %w", tvdbID, err)
		}
		ords, err := s.tvdb.Orderings(ctx, tvdbID, types)
		if err != nil {
			return err
		}
		tvdbSeries = ser
		orderings = append(orderings, ords...)
		if tmdbID == 0 {
			tmdbID = ser.TMDBID
		}
		return nil
	}

	if s.tvdb.Configured() {
		if tvdbID == 0 {
			if tvdbID, err = s.tvdb.SearchSeries(ctx, series.Title, series.Year); err != nil {
				return fmt.Errorf("tvdb search: %w", err)
			}
		}
		if tvdbID != 0 {
			if err := fetchTVDB(); err != nil {
				return err
			}
		}
	}
	if s.tmdb.Configured() {
		if tmdbID == 0 && tvdbID != 0 {
			if tmdbID, err = s.tmdb.FindTV(ctx, strconv.FormatInt(tvdbID, 10), "tvdb_id"); err != nil {
				return fmt.Errorf("tmdb find: %w", err)
			}
		}
		if tmdbID == 0 && series.IMDBID != "" {
			if tmdbID, err = s.tmdb.FindTV(ctx, series.IMDBID, "imdb_id"); err != nil {
				return fmt.Errorf("tmdb find: %w", err)
			}
		}
		if tmdbID == 0 && tvdbSeries == nil {
			if tmdbID, err = s.tmdb.SearchTV(ctx, series.Title, series.Year); err != nil {
				return fmt.Errorf("tmdb search: %w", err)
			}
		}
		if tmdbID != 0 {
			ser, ords, err := s.tmdb.TV(ctx, tmdbID)
			if err != nil {
				return fmt.Errorf("tmdb series %d: %w", tmdbID, err)
			}
			tmdbSeries = ser
			orderings = append(orderings, ords...)
			if tvdbSeries == nil && ser.TVDBID != 0 && s.tvdb.Configured() {
				tvdbID = ser.TVDBID
				if err := fetchTVDB(); err != nil {
					return err
				}
			}
		}
	}

	if tvdbSeries == nil && tmdbSeries == nil {
		log.Info("no match found for series")
		s.update(lib.ID, func(st *Status) { st.Unmatched++ })
		return s.store.SetMetadataState(ctx, series.ID, "unmatched")
	}

	// Episodes are identified by the provider the filenames are numbered with.
	canon := "tmdb"
	if tvdbSeries != nil && (strings.HasPrefix(lib.SourceOrdering, "tvdb") || tmdbSeries == nil) {
		canon = "tvdb"
	}
	if canon == "tmdb" && strings.HasPrefix(lib.SourceOrdering, "tvdb") {
		log.Warn("series not found on TVDB; reading its file names using TMDB numbering")
	}

	// Series metadata: TMDB's is usually richer and has better artwork, so prefer it.
	primary, secondary := tmdbSeries, tvdbSeries
	if primary == nil {
		primary, secondary = tvdbSeries, nil
	}
	it := *series
	it.Title, it.OriginalTitle, it.Overview = primary.Title, primary.OriginalTitle, primary.Overview
	it.ReleaseDate, it.Year = primary.FirstAired, primary.Year
	it.PosterPath, it.BackdropPath = primary.PosterPath, primary.BackdropPath
	it.IMDBID = primary.IMDBID
	if secondary != nil {
		if it.PosterPath == "" {
			it.PosterPath = secondary.PosterPath
		}
		if it.Overview == "" {
			it.Overview = secondary.Overview
		}
		if it.IMDBID == "" {
			it.IMDBID = secondary.IMDBID
		}
	}
	it.TVDBID, it.TMDBID = tvdbID, tmdbID
	if tvdbSeries == nil {
		it.TVDBID = 0
	}
	if tmdbSeries == nil {
		it.TMDBID = 0
	}
	applyDetails(&it, primary.Details)
	it.MetadataState = "matched"
	if err := s.store.UpdateMetadata(ctx, &it); err != nil {
		return err
	}
	if err := s.saveDetails(ctx, it.ID, primary.Details); err != nil {
		return err
	}

	// Canonical episodes, in first-seen order.
	canonEps := map[int64]*metadata.Episode{}
	var canonOrder []int64
	otherEps := map[int64]*metadata.Episode{}
	for _, o := range orderings {
		for _, e := range o.Entries {
			if o.Provider() == canon {
				if canonEps[e.Episode.ID] == nil {
					canonEps[e.Episode.ID] = e.Episode
					canonOrder = append(canonOrder, e.Episode.ID)
				}
			} else if otherEps[e.Episode.ID] == nil {
				otherEps[e.Episode.ID] = e.Episode
			}
		}
	}

	if !deep {
		if deep, err = s.store.DisplaysProvider(ctx, series.ID, otherProvider(canon)); err != nil {
			return err
		}
	}
	otherToCanon, err := s.mapEpisodes(ctx, series.ID, canon, tmdbID, canonEps, otherEps, deep)
	if err != nil {
		return err
	}
	if len(otherEps) > 0 {
		log.Info("mapped episodes between providers", "canonical", canon,
			"mapped", len(otherToCanon), "of", len(otherEps), "deep", deep)
	}
	canonToOther := map[int64]int64{}
	for o, c := range otherToCanon {
		if prev, ok := canonToOther[c]; !ok || o < prev {
			canonToOther[c] = o
		}
	}

	// Positions that file-only episodes may have been created at, so they can be adopted (keeping
	// their id and watch history) now that the provider knows them.
	sourcePos := map[int64]string{}
	srcSource := pickSource(lib.SourceOrdering, func(src string) bool {
		for _, o := range orderings {
			if o.Source == src {
				return true
			}
		}
		return false
	})
	for _, o := range orderings {
		if o.Source == srcSource || o.Source == "tvdb_absolute" {
			for _, e := range o.Entries {
				if _, ok := sourcePos[e.Episode.ID]; ok {
					continue
				}
				if o.Source == "tvdb_absolute" {
					sourcePos[e.Episode.ID] = fileEpisodeKey(series.ID, true, 0, e.Number)
				} else {
					sourcePos[e.Episode.ID] = fileEpisodeKey(series.ID, false, e.Season, e.Number)
				}
			}
		}
	}

	itemFor := map[int64]string{}
	for _, cid := range canonOrder {
		e := canonEps[cid]
		key := episodeKey(series.ID, canon, cid)
		if fk, ok := sourcePos[cid]; ok {
			if err := s.adoptFileEpisode(ctx, lib.ID, fk, key); err != nil {
				return err
			}
		}
		itemID, _, err := s.store.EnsureItem(ctx, &store.Item{
			LibraryID: lib.ID, Kind: "episode", ParentID: series.ID, Title: e.Title,
		}, key)
		if err != nil {
			return err
		}
		ep := store.Item{ID: itemID, Title: e.Title, Overview: e.Overview, ReleaseDate: e.AirDate,
			Year: yearFromDate(e.AirDate), RuntimeMin: e.RuntimeMin, PosterPath: e.Still, MetadataState: "matched"}
		if canon == "tvdb" {
			ep.TVDBID, ep.TMDBID = cid, canonToOther[cid]
		} else {
			ep.TMDBID, ep.TVDBID = cid, canonToOther[cid]
		}
		if err := s.store.UpdateMetadata(ctx, &ep); err != nil {
			return err
		}
		itemFor[cid] = itemID
	}

	keep := map[int64]bool{}
	for _, o := range orderings {
		var entries []store.OrderingEntry
		unmapped := 0
		for _, e := range o.Entries {
			cid := e.Episode.ID
			if o.Provider() != canon {
				c, ok := otherToCanon[cid]
				if !ok {
					unmapped++
					continue
				}
				cid = c
			}
			entries = append(entries, store.OrderingEntry{EpisodeID: itemFor[cid], Season: e.Season, Episode: e.Number})
		}
		oid, err := s.store.UpsertOrdering(ctx, series.ID, o.Source, o.SourceRef, o.Name, o.Description)
		if err != nil {
			return err
		}
		if err := s.store.ReplaceOrdering(ctx, oid, o.Seasons, entries); err != nil {
			return err
		}
		keep[oid] = true
		if unmapped > 0 {
			log.Info("episodes in ordering without a matching episode", "ordering", o.Name, "count", unmapped)
		}
	}

	// Drop orderings a provider no longer offers, and the file-name fallback ordering.
	existing, err := s.store.ListOrderings(ctx, series.ID)
	if err != nil {
		return err
	}
	for _, o := range existing {
		if keep[o.ID] || o.Source == "custom" {
			continue
		}
		stale := o.Source == "file" ||
			(strings.HasPrefix(o.Source, "tvdb") && tvdbSeries != nil) ||
			(strings.HasPrefix(o.Source, "tmdb") && tmdbSeries != nil)
		if stale {
			if err := s.store.DeleteOrdering(ctx, o.ID); err != nil {
				return err
			}
		}
	}

	s.update(lib.ID, func(st *Status) { st.Matched++ })
	log.Info("series synced", "episodes", len(canonEps), "orderings", len(orderings))
	return s.store.MarkSynced(ctx, series.ID)
}

// mapEpisodes maps the non-canonical provider's episodes onto canonical ones: ids remembered from
// earlier syncs, then same air date and title. With deep set it continues with TMDB's record of
// TVDB ids (an API call per episode), then a unique title, then a unique air date. Several
// episodes may map to one (a two-part pilot shown as one episode).
func (s *Scanner) mapEpisodes(ctx context.Context, seriesID, canon string, tmdbSeriesID int64,
	canonEps, otherEps map[int64]*metadata.Episode, deep bool) (map[int64]int64, error) {
	out := map[int64]int64{}
	if len(otherEps) == 0 {
		return out, nil
	}

	kids, err := s.store.Children(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	for _, k := range kids {
		c, o := k.TVDBID, k.TMDBID
		if canon == "tmdb" {
			c, o = k.TMDBID, k.TVDBID
		}
		if c != 0 && o != 0 && canonEps[c] != nil && otherEps[o] != nil {
			out[o] = c
		}
	}

	type dateTitle struct{ date, title string }
	byDT := map[dateTitle][]int64{}
	byTitle := map[string][]int64{}
	byDate := map[string][]int64{}
	for id, e := range canonEps {
		t := metadata.Normalize(e.Title)
		if e.AirDate != "" && t != "" {
			byDT[dateTitle{e.AirDate, t}] = append(byDT[dateTitle{e.AirDate, t}], id)
		}
		if t != "" {
			byTitle[t] = append(byTitle[t], id)
		}
		if e.AirDate != "" {
			byDate[e.AirDate] = append(byDate[e.AirDate], id)
		}
	}
	unresolved := func() []int64 {
		var ids []int64
		for id := range otherEps {
			if _, ok := out[id]; !ok {
				ids = append(ids, id)
			}
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		return ids
	}

	for _, oid := range unresolved() {
		e := otherEps[oid]
		if c := byDT[dateTitle{e.AirDate, metadata.Normalize(e.Title)}]; len(c) == 1 {
			out[oid] = c[0]
		}
	}

	if !deep {
		return out, nil
	}

	if s.tmdb.Configured() && tmdbSeriesID != 0 && len(unresolved()) > 0 {
		if canon == "tvdb" {
			for _, oid := range unresolved() {
				e := otherEps[oid]
				tv, err := s.tmdb.EpisodeTVDBID(ctx, tmdbSeriesID, e.AiredSeason, e.AiredNumber)
				if err != nil {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					s.log.Warn("tmdb episode lookup failed", "err", err)
					break
				}
				if tv != 0 && canonEps[tv] != nil {
					out[oid] = tv
				}
			}
		} else {
			// Canonical episodes are TMDB's: learn their TVDB ids until every TVDB episode is placed.
			taken := map[int64]bool{}
			for _, c := range out {
				taken[c] = true
			}
			ids := make([]int64, 0, len(canonEps))
			for id := range canonEps {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			for _, cid := range ids {
				if len(unresolved()) == 0 {
					break
				}
				if taken[cid] {
					continue
				}
				e := canonEps[cid]
				tv, err := s.tmdb.EpisodeTVDBID(ctx, tmdbSeriesID, e.AiredSeason, e.AiredNumber)
				if err != nil {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					s.log.Warn("tmdb episode lookup failed", "err", err)
					break
				}
				if _, done := out[tv]; tv != 0 && otherEps[tv] != nil && !done {
					out[tv] = cid
				}
			}
		}
	}

	for _, oid := range unresolved() {
		if c := byTitle[metadata.Normalize(otherEps[oid].Title)]; len(c) == 1 {
			out[oid] = c[0]
		}
	}
	for _, oid := range unresolved() {
		if c := byDate[otherEps[oid].AirDate]; otherEps[oid].AirDate != "" && len(c) == 1 {
			out[oid] = c[0]
		}
	}
	return out, nil
}

func otherProvider(p string) string {
	if p == "tvdb" {
		return "tmdb"
	}
	return "tvdb"
}

// adoptFileEpisode re-keys a file-only episode (created before metadata knew about it) to its
// provider key, unless an item with that key already exists.
func (s *Scanner) adoptFileEpisode(ctx context.Context, libraryID int64, fileKey, providerKey string) error {
	fileItem, err := s.store.GetItemByKey(ctx, libraryID, fileKey)
	if err != nil || fileItem == nil {
		return err
	}
	existing, err := s.store.GetItemByKey(ctx, libraryID, providerKey)
	if err != nil || existing != nil {
		return err
	}
	return s.store.RekeyItem(ctx, fileItem.ID, providerKey)
}

// linkEpisodeFiles attaches files to episodes using the series' source ordering (how its files are
// numbered). Files naming episodes the ordering doesn't contain are counted as unknown; when
// createMissing is set they get file-only episodes so they're still visible and playable.
func (s *Scanner) linkEpisodeFiles(ctx context.Context, lib *store.Library, seriesID string, files []episodeFile,
	createMissing bool) (unknown int, err error) {
	orderings, err := s.store.ListOrderings(ctx, seriesID)
	if err != nil {
		return 0, err
	}
	bySource := map[string]int64{}
	for _, o := range orderings {
		if o.SourceRef == "" {
			bySource[o.Source] = o.ID
		}
	}
	srcID := bySource[pickSource(lib.SourceOrdering, func(src string) bool { return bySource[src] != 0 })]
	if srcID == 0 {
		if srcID, err = s.store.UpsertOrdering(ctx, seriesID, "file", "", "File Order", "From file names"); err != nil {
			return 0, err
		}
	}

	entries, err := s.store.OrderingEntries(ctx, srcID)
	if err != nil {
		return 0, err
	}
	type pos struct{ season, episode int }
	at := map[pos][]string{}
	for _, e := range entries {
		at[pos{e.Season, e.Episode}] = append(at[pos{e.Season, e.Episode}], e.EpisodeID)
	}

	// Absolute numbers: TVDB's absolute order if the series has one, otherwise count through the
	// source ordering's regular seasons.
	absolute := map[int][]string{}
	if absID := bySource["tvdb_absolute"]; absID != 0 {
		absEntries, err := s.store.OrderingEntries(ctx, absID)
		if err != nil {
			return 0, err
		}
		for _, e := range absEntries {
			absolute[e.Episode] = append(absolute[e.Episode], e.EpisodeID)
		}
	} else {
		n, seen := 0, map[string]bool{}
		for _, e := range entries { // already sorted by season, episode
			if e.Season > 0 && !seen[e.EpisodeID] {
				seen[e.EpisodeID] = true
				n++
				absolute[n] = []string{e.EpisodeID}
			}
		}
	}

	var byDate map[string][]string
	for _, f := range files {
		var ids []string
		missing := false
		switch {
		case f.ep.AirDate != "":
			if byDate == nil {
				byDate = map[string][]string{}
				kids, err := s.store.Children(ctx, seriesID)
				if err != nil {
					return 0, err
				}
				for _, k := range kids {
					if k.ReleaseDate != "" {
						byDate[k.ReleaseDate] = append(byDate[k.ReleaseDate], k.ID)
					}
				}
			}
			if c := byDate[f.ep.AirDate]; len(c) > 0 {
				ids = c[:1]
			} else {
				missing = true
			}
		default:
			for _, n := range f.ep.Episodes {
				var c []string
				if f.ep.Absolute {
					c = absolute[n]
				} else {
					c = at[pos{f.ep.Season, n}]
				}
				if len(c) > 0 {
					ids = append(ids, c...)
					continue
				}
				missing = true
				if createMissing {
					id, err := s.fileOnlyEpisode(ctx, lib.ID, seriesID, srcID, f.ep.Absolute, f.ep.Season, n)
					if err != nil {
						return 0, err
					}
					ids = append(ids, id)
				}
			}
		}
		if missing {
			unknown++
		}
		if err := s.store.SetFileItems(ctx, f.fileID, dedupe(ids), 0); err != nil {
			return 0, err
		}
		// A "<file>-thumb.jpg" beside the episode is its still.
		thumb := episodeThumb(f.path)
		for _, id := range dedupe(ids) {
			if err := s.store.SetLocalArt(ctx, id, thumb, "", ""); err != nil {
				return 0, err
			}
		}
	}
	return unknown, nil
}

// fileOnlyEpisode returns an episode known only from its file name, creating it if needed, and
// places it in the source ordering at the position the file name gives.
func (s *Scanner) fileOnlyEpisode(ctx context.Context, libraryID int64, seriesID string, orderingID int64,
	absolute bool, season, number int) (string, error) {
	if absolute {
		season = 1
	}
	key := fileEpisodeKey(seriesID, absolute, season, number)
	id, created, err := s.store.EnsureItem(ctx, &store.Item{
		LibraryID: libraryID, Kind: "episode", ParentID: seriesID, Title: "Episode " + strconv.Itoa(number),
	}, key)
	if err != nil {
		return "", err
	}
	if created {
		if err := s.store.SetMetadataState(ctx, id, "unmatched"); err != nil {
			return "", err
		}
	}
	return id, s.store.AddEntry(ctx, orderingID, id, season, number)
}

// pickSource chooses the ordering files are numbered in: the library's setting if the series has
// it, otherwise the closest available fallback.
func pickSource(preferred string, has func(string) bool) string {
	for _, src := range []string{preferred, "tvdb_aired", "tmdb_aired", "file"} {
		if has(src) {
			return src
		}
	}
	return ""
}

func seriesMatchKey(n MovieName) string {
	switch {
	case n.TVDBID != 0:
		return "series:tvdb:" + strconv.FormatInt(n.TVDBID, 10)
	case n.TMDBID != 0:
		return "series:tmdb:" + strconv.FormatInt(n.TMDBID, 10)
	case n.IMDBID != "":
		return "series:imdb:" + n.IMDBID
	}
	return "series:title:" + metadata.Normalize(n.Title) + ":" + strconv.Itoa(n.Year)
}

func episodeKey(seriesID, provider string, id int64) string {
	return "ep:" + seriesID + ":" + provider + ":" + strconv.FormatInt(id, 10)
}

func fileEpisodeKey(seriesID string, absolute bool, season, number int) string {
	if absolute {
		return fmt.Sprintf("ep:%s:file:a%d", seriesID, number)
	}
	return fmt.Sprintf("ep:%s:file:s%de%d", seriesID, season, number)
}

func yearFromDate(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

func dedupe(ids []string) []string {
	seen := map[string]bool{}
	out := ids[:0:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

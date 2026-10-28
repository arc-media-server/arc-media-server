package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"ams/internal/store"
)

// Playlists belong to a profile: movies and episodes, played in order.

type playlistDTO struct {
	store.Playlist
	// The first few titles' posters, for the playlist's picture.
	Images []string `json:"images"`
	// A smart playlist's filter.
	SmartFilter *rowFilter `json:"smart_filter,omitempty"`
}

func toPlaylistDTO(p store.Playlist) playlistDTO {
	d := playlistDTO{Playlist: p, Images: []string{}}
	if p.Filter != "" {
		var f rowFilter
		if json.Unmarshal([]byte(p.Filter), &f) == nil {
			d.SmartFilter = &f
		}
	}
	return d
}

// smartLimit is the most titles a smart playlist holds.
const smartLimit = 500

// playlistContents is a playlist's entries and titles in order: stored ones, or a smart playlist's
// (entry ids 0), worked out now for the profile.
func (s *Server) playlistContents(r *http.Request, p *store.Playlist) ([]store.PlaylistEntry, []store.Item, error) {
	if p.Filter == "" {
		return s.store.PlaylistEntries(r.Context(), p.ID, access(r))
	}
	ctx := r.Context()
	var f rowFilter
	if err := json.Unmarshal([]byte(p.Filter), &f); err != nil {
		return nil, nil, nil
	}
	lib, err := s.store.GetLibrary(ctx, f.LibraryID)
	if err != nil || lib == nil || !access(r).LibraryAllowed(lib.ID) {
		return nil, nil, err
	}
	q := store.ItemQuery{LibraryID: lib.ID, Kind: "movie", Genre: f.Genre, Language: f.Language, Label: f.Label, Decade: f.Decade, Resolution: f.Resolution,
		Unwatched: f.Unwatched, AddedWithinDays: f.AddedWithin, MinRuntime: f.MinRuntime, MaxRuntime: f.MaxRuntime,
		MinRating: f.MinRating, Sort: f.Sort, Access: access(r), UserID: currentUserID(r), Limit: smartLimit}
	if lib.Kind == "shows" {
		q.Kind = "series"
	}
	found, _, err := s.store.ListItems(ctx, q)
	if err != nil || lib.Kind != "shows" {
		return make([]store.PlaylistEntry, len(found)), found, err
	}
	// Shows play as their episodes, in the profile's order (only unwatched ones when asked).
	var ids []string
	for i := range found {
		eps, err := s.orderedEpisodeIDs(r, &found[i])
		if err != nil {
			return nil, nil, err
		}
		ids = append(ids, eps...)
		if len(ids) >= smartLimit*4 {
			break
		}
	}
	if f.Unwatched {
		data, err := s.store.UserDataFor(ctx, currentUserID(r), ids)
		if err != nil {
			return nil, nil, err
		}
		ids = slices.DeleteFunc(ids, func(id string) bool { return data[id].Played })
	}
	if len(ids) > smartLimit {
		ids = ids[:smartLimit]
	}
	byID, err := s.store.ItemsByID(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	items := make([]store.Item, 0, len(ids))
	for _, id := range ids {
		if it, ok := byID[id]; ok {
			items = append(items, it)
		}
	}
	return make([]store.PlaylistEntry, len(items)), items, nil
}

// smartFilter checks a smart playlist's filter: a library the profile can see.
func (s *Server) smartFilter(r *http.Request, f *rowFilter) (string, error) {
	lib, err := s.store.GetLibrary(r.Context(), f.LibraryID)
	if err != nil {
		return "", err
	}
	if lib == nil || !access(r).LibraryAllowed(lib.ID) || lib.Kind == "videos" {
		return "", errMsg("choose a movie or TV library for the playlist's rules")
	}
	raw, _ := json.Marshal(f)
	return string(raw), nil
}

// notSmart refuses changing a smart playlist's titles by hand.
func notSmart(w http.ResponseWriter, p *store.Playlist) bool {
	if p.Filter != "" {
		writeError(w, http.StatusConflict, "this is a smart playlist: change its rules instead")
		return false
	}
	return true
}

type playlistEntryDTO struct {
	EntryID int64   `json:"entry_id"`
	Item    itemDTO `json:"item"`
}

// playlistFromPath returns the profile's playlist named in the path.
func (s *Server) playlistFromPath(w http.ResponseWriter, r *http.Request) (*store.Playlist, bool) {
	p, err := s.store.GetPlaylist(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return nil, false
	}
	if p == nil || p.UserID != currentUserID(r) {
		writeError(w, http.StatusNotFound, "playlist not found")
		return nil, false
	}
	return p, true
}

// playlistItems describes a playlist's titles: episodes with their show and place in it.
func (s *Server) playlistItems(r *http.Request, p *store.Playlist) ([]playlistEntryDTO, error) {
	ctx := r.Context()
	entries, items, err := s.playlistContents(r, p)
	if err != nil {
		return nil, err
	}
	out := make([]playlistEntryDTO, 0, len(items))
	series := map[string]*store.Item{}
	positions := map[string]map[string][2]int{}
	for i, it := range items {
		d := toDTO(it)
		if it.Kind == "episode" {
			sr, ok := series[it.ParentID]
			if !ok {
				sr, _ = s.store.GetItem(ctx, it.ParentID)
				series[it.ParentID] = sr
				if sr != nil {
					positions[sr.ID], _, _ = s.orderPositions(ctx, currentUserID(r), sr)
				}
			}
			d = s.episodeDTO(it, sr, positions[it.ParentID])
		}
		out = append(out, playlistEntryDTO{EntryID: entries[i].ID, Item: d})
	}
	ptrsOut := make([]*itemDTO, len(out))
	for i := range out {
		ptrsOut[i] = &out[i].Item
	}
	if err := s.fillUserData(r, ptrsOut); err != nil {
		return nil, err
	}
	return out, nil
}

// listPlaylists: GET /api/playlists is the profile's playlists.
func (s *Server) listPlaylists(w http.ResponseWriter, r *http.Request) {
	lists, err := s.store.Playlists(r.Context(), currentUserID(r), access(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]playlistDTO, 0, len(lists))
	for _, p := range lists {
		d := toPlaylistDTO(p)
		if _, items, err := s.playlistContents(r, &p); err == nil {
			if p.Filter != "" {
				d.Count, d.DurationMS = len(items), 0
				for _, it := range items {
					d.DurationMS += int64(it.RuntimeMin) * 60000
				}
			}
			for _, it := range items {
				img := toDTO(it).Images["primary"]
				if it.Kind == "episode" {
					// An episode's poster is its show's.
					img = "/api/items/" + it.ParentID + "/images/primary"
				}
				if img != "" && !slices.Contains(d.Images, img) {
					d.Images = append(d.Images, img)
				}
				if len(d.Images) == 4 {
					break
				}
			}
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

// getPlaylist: GET /api/playlists/{id} is a playlist and its titles in order.
func (s *Server) getPlaylist(w http.ResponseWriter, r *http.Request) {
	p, ok := s.playlistFromPath(w, r)
	if !ok {
		return
	}
	entries, err := s.playlistItems(r, p)
	if err != nil {
		s.fail(w, err)
		return
	}
	p.Count = len(entries)
	for _, e := range entries {
		p.DurationMS += int64(e.Item.RuntimeMin) * 60000
	}
	writeJSON(w, http.StatusOK, map[string]any{"playlist": toPlaylistDTO(*p), "entries": entries})
}

// playlistItemIDs turns movies, episodes and shows (all their episodes, in the profile's episode
// order) into the titles to add.
func (s *Server) playlistItemIDs(r *http.Request, ids []string) ([]string, error) {
	var out []string
	for _, id := range ids {
		it, err := s.store.GetItem(r.Context(), id)
		if err != nil {
			return nil, err
		}
		if it == nil {
			return nil, errMsg("a title to add wasn't found")
		}
		switch it.Kind {
		case "movie", "episode":
			if ok, err := s.store.CanSee(r.Context(), it, access(r)); err != nil || !ok {
				return nil, errMsg("a title to add wasn't found")
			}
			out = append(out, it.ID)
		case "series":
			eps, err := s.orderedEpisodeIDs(r, it)
			if err != nil {
				return nil, err
			}
			out = append(out, eps...)
		default:
			return nil, errMsg("only movies, episodes and shows can go in a playlist")
		}
	}
	return out, nil
}

// orderedEpisodeIDs lists a show's episodes on disk in the profile's episode order, then any the
// order doesn't place.
func (s *Server) orderedEpisodeIDs(r *http.Request, series *store.Item) ([]string, error) {
	ctx := r.Context()
	available, err := s.store.AvailableChildren(ctx, series.ID)
	if err != nil {
		return nil, err
	}
	_, entries, err := s.orderPositions(ctx, currentUserID(r), series)
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	// Seasons first, then specials (an episode in both, like a pilot, goes where its season puts it).
	for _, specials := range []bool{false, true} {
		for _, e := range entries {
			if (e.Season == 0 && !e.InlineSpecial) == specials && available[e.EpisodeID] && !seen[e.EpisodeID] {
				seen[e.EpisodeID] = true
				out = append(out, e.EpisodeID)
			}
		}
	}
	var rest []string
	for id := range available {
		if !seen[id] {
			rest = append(rest, id)
		}
	}
	slices.Sort(rest)
	return append(out, rest...), nil
}

// createPlaylist: POST /api/playlists {"title", "item_ids"} makes a playlist for the profile; with
// "filter" (a library filter, as for a smart collection) it's a smart playlist kept up to date.
func (s *Server) createPlaylist(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Title   string     `json:"title"`
		ItemIDs []string   `json:"item_ids"`
		Filter  *rowFilter `json:"filter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b.Title = strings.TrimSpace(b.Title)
	if b.Title == "" || len([]rune(b.Title)) > 100 {
		writeError(w, http.StatusBadRequest, "give the playlist a name of up to 100 characters")
		return
	}
	ids, err := s.playlistItemIDs(r, b.ItemIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filter := ""
	if b.Filter != nil {
		if filter, err = s.smartFilter(r, b.Filter); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		ids = nil
	}
	p, err := s.store.CreatePlaylist(r.Context(), currentUserID(r), b.Title)
	if err != nil {
		s.fail(w, err)
		return
	}
	if filter != "" {
		if err := s.store.SetPlaylistFilter(r.Context(), p.ID, filter); err != nil {
			s.fail(w, err)
			return
		}
		p.Filter = filter
		_, items, _ := s.playlistContents(r, p)
		p.Count = len(items)
		writeJSON(w, http.StatusCreated, toPlaylistDTO(*p))
		return
	}
	if len(ids) > 0 {
		if err := s.store.AddToPlaylist(r.Context(), p.ID, ids); err != nil {
			s.fail(w, err)
			return
		}
	}
	p.Count = len(ids)
	writeJSON(w, http.StatusCreated, toPlaylistDTO(*p))
}

// updatePlaylist: PATCH /api/playlists/{id} {"title", "filter"} renames a playlist, or changes a
// smart playlist's rules.
func (s *Server) updatePlaylist(w http.ResponseWriter, r *http.Request) {
	p, ok := s.playlistFromPath(w, r)
	if !ok {
		return
	}
	var b struct {
		Title  *string    `json:"title"`
		Filter *rowFilter `json:"filter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || (b.Title != nil && strings.TrimSpace(*b.Title) == "") {
		writeError(w, http.StatusBadRequest, `send {"title": "..."} or a smart playlist's {"filter": {...}}`)
		return
	}
	if b.Title != nil {
		if err := s.store.RenamePlaylist(r.Context(), p.ID, strings.TrimSpace(*b.Title)); err != nil {
			s.fail(w, err)
			return
		}
	}
	if b.Filter != nil {
		if p.Filter == "" {
			writeError(w, http.StatusConflict, "only a smart playlist has rules")
			return
		}
		filter, err := s.smartFilter(r, b.Filter)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.store.SetPlaylistFilter(r.Context(), p.ID, filter); err != nil {
			s.fail(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// deletePlaylist: DELETE /api/playlists/{id}. Deletes a playlist and its entries.
func (s *Server) deletePlaylist(w http.ResponseWriter, r *http.Request) {
	p, ok := s.playlistFromPath(w, r)
	if !ok {
		return
	}
	if err := s.store.DeletePlaylist(r.Context(), p.ID); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// playlistAdd: POST /api/playlists/{id}/items {"item_ids"} adds movies, episodes or whole shows.
func (s *Server) playlistAdd(w http.ResponseWriter, r *http.Request) {
	p, ok := s.playlistFromPath(w, r)
	if !ok {
		return
	}
	if !notSmart(w, p) {
		return
	}
	var b struct {
		ItemIDs []string `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || len(b.ItemIDs) == 0 {
		writeError(w, http.StatusBadRequest, `send {"item_ids": [...]}`)
		return
	}
	ids, err := s.playlistItemIDs(r, b.ItemIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.AddToPlaylist(r.Context(), p.ID, ids); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"added": len(ids)})
}

// playlistRemove: DELETE /api/playlists/{id}/entries/{entry} takes an entry out of a playlist.
func (s *Server) playlistRemove(w http.ResponseWriter, r *http.Request) {
	p, ok := s.playlistFromPath(w, r)
	if !ok {
		return
	}
	if !notSmart(w, p) {
		return
	}
	entry, err := strconv.ParseInt(r.PathValue("entry"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad entry")
		return
	}
	if err := s.store.RemovePlaylistEntry(r.Context(), p.ID, entry); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// playlistOrder: PUT /api/playlists/{id}/order {"entry_ids"} puts a playlist in order.
func (s *Server) playlistOrder(w http.ResponseWriter, r *http.Request) {
	p, ok := s.playlistFromPath(w, r)
	if !ok {
		return
	}
	if !notSmart(w, p) {
		return
	}
	var b struct {
		EntryIDs []int64 `json:"entry_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, `send {"entry_ids": [...]}`)
		return
	}
	if err := s.store.OrderPlaylist(r.Context(), p.ID, b.EntryIDs); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

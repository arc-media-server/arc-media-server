package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"ams/internal/artwork"
	"ams/internal/metadata"
	"ams/internal/scanner"
	"ams/internal/store"
)

// Collections group a library's titles: automatic ones from TMDB, manual ones the admin fills and
// orders, and smart ones that hold whatever their filter matches.

type collectionDTO struct {
	store.Collection
	Images map[string]string `json:"images"`
	// Smart collections: the filter, decoded.
	SmartFilter *rowFilter `json:"smart_filter,omitempty"`
}

// collectionRef names a collection a title is in.
type collectionRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

func (s *Server) collectionDTO(c store.Collection, first *store.Item) collectionDTO {
	d := collectionDTO{Collection: c, Images: map[string]string{}}
	base := "/api/collections/" + c.ID + "/images/"
	if c.PosterPath != "" {
		d.Images["primary"] = base + "primary?v=" + artwork.Key(artwork.Source("collection", "primary", c.PosterPath))
	}
	if c.BackdropPath != "" {
		d.Images["backdrop"] = base + "backdrop?v=" + artwork.Key(artwork.Source("collection", "backdrop", c.BackdropPath))
	}
	// Without artwork of its own, a collection shows its first title's.
	if first != nil {
		fd := toDTO(*first)
		for _, typ := range []string{"primary", "backdrop"} {
			if d.Images[typ] == "" && fd.Images[typ] != "" {
				d.Images[typ] = fd.Images[typ]
			}
		}
	}
	if c.Kind == "smart" && c.Filter != "" {
		var f rowFilter
		if json.Unmarshal([]byte(c.Filter), &f) == nil {
			d.SmartFilter = &f
		}
	}
	return d
}

// collectionItems returns the titles in a collection the profile can see, and how many there are.
func (s *Server) collectionItems(r *http.Request, c *store.Collection, limit int) ([]store.Item, int, error) {
	ctx := r.Context()
	if c.Kind != "smart" {
		items, err := s.store.CollectionItems(ctx, c, access(r))
		if err != nil {
			return nil, 0, err
		}
		total := len(items)
		if limit > 0 && len(items) > limit {
			items = items[:limit]
		}
		return items, total, nil
	}
	var f rowFilter
	if err := json.Unmarshal([]byte(c.Filter), &f); err != nil {
		return nil, 0, nil
	}
	lib, err := s.store.GetLibrary(ctx, c.LibraryID)
	if err != nil || lib == nil {
		return nil, 0, err
	}
	q := store.ItemQuery{LibraryID: c.LibraryID, Kind: "movie", Genre: f.Genre, Language: f.Language, Label: f.Label, Decade: f.Decade, Resolution: f.Resolution,
		Unwatched: f.Unwatched, AddedWithinDays: f.AddedWithin, MinRuntime: f.MinRuntime, MaxRuntime: f.MaxRuntime,
		MinRating: f.MinRating, Sort: f.Sort, Access: access(r), UserID: currentUserID(r), Limit: 500}
	if lib.Kind == "shows" {
		q.Kind = "series"
	}
	if limit > 0 {
		q.Limit = limit
	}
	return s.store.ListItems(ctx, q)
}

// listCollections: GET /api/libraries/{id}/collections lists a library's collections.
func (s *Server) listCollections(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	cols, err := s.store.Collections(r.Context(), lib.ID, access(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	out := []collectionDTO{}
	for _, c := range cols {
		first, total, err := s.collectionItems(r, &c, 1)
		if err != nil {
			s.fail(w, err)
			return
		}
		c.Count = total
		if c.Kind == "smart" && total == 0 {
			continue // nothing matches (for this profile)
		}
		var f *store.Item
		if len(first) > 0 {
			f = &first[0]
		}
		out = append(out, s.collectionDTO(c, f))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) collectionFromPath(w http.ResponseWriter, r *http.Request) (*store.Collection, bool) {
	c, err := s.store.GetCollection(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return nil, false
	}
	if c == nil || c.Hidden || !access(r).LibraryAllowed(c.LibraryID) {
		writeError(w, http.StatusNotFound, "collection not found")
		return nil, false
	}
	return c, true
}

// getCollection: GET /api/collections/{id} is a collection and its titles.
func (s *Server) getCollection(w http.ResponseWriter, r *http.Request) {
	c, ok := s.collectionFromPath(w, r)
	if !ok {
		return
	}
	// An automatic collection's description comes from TMDB the first time it's opened.
	if c.Kind == "auto" && c.Overview == "" && !slices.Contains(c.LockedFields, "overview") {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		if info, err := s.scanner.CollectionInfo(ctx, c.TMDBID); err == nil && info != nil {
			if s.store.SetCollectionInfo(r.Context(), c.ID, info.Overview, info.PosterPath, info.BackdropPath) == nil {
				c.Overview = info.Overview
				if info.PosterPath != "" {
					c.PosterPath = info.PosterPath
				}
				if info.BackdropPath != "" {
					c.BackdropPath = info.BackdropPath
				}
			}
		}
		cancel()
	}
	items, total, err := s.collectionItems(r, c, 0)
	if err != nil {
		s.fail(w, err)
		return
	}
	c.Count = total
	if total == 0 && c.Kind == "auto" {
		writeError(w, http.StatusNotFound, "collection not found")
		return
	}
	out := toDTOs(items)
	if err := s.fillUserData(r, ptrs(out)); err != nil {
		s.fail(w, err)
		return
	}
	var first *store.Item
	if len(items) > 0 {
		first = &items[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{"collection": s.collectionDTO(*c, first), "items": out})
}

// collectionImage: GET /api/collections/{id}/images/{primary|backdrop} is a collection's poster or background.
func (s *Server) collectionImage(w http.ResponseWriter, r *http.Request) {
	c, ok := s.collectionFromPath(w, r)
	if !ok {
		return
	}
	path := c.PosterPath
	typ := r.PathValue("type")
	if typ == "backdrop" {
		path = c.BackdropPath
	} else if typ != "primary" {
		writeError(w, http.StatusBadRequest, "unknown image type")
		return
	}
	if path == "" {
		writeError(w, http.StatusNotFound, "no image")
		return
	}
	// Uploaded artwork lives in the data folder.
	if name, ok := strings.CutPrefix(path, metadata.UploadPrefix); ok {
		http.ServeFile(w, r, filepath.Join(s.uploadsDir, filepath.Base(name)))
		return
	}
	s.serveArtwork(w, r, artwork.Source("collection", typ, path))
}

type collectionBody struct {
	LibraryID int64   `json:"library_id"`
	Kind      string  `json:"kind"`
	Title     *string `json:"title"`
	SortTitle *string `json:"sort_title"`
	Overview  *string `json:"overview"`
	// Replaces the locked fields (fields sent are locked as well).
	LockedFields *[]string  `json:"locked_fields"`
	Filter       *rowFilter `json:"filter"`
	ItemIDs      []string   `json:"item_ids"`
}

// createCollection: POST /api/collections {"library_id", "kind": "manual"|"smart", "title",
// "overview", "filter" (smart), "item_ids" (manual)}.
func (s *Server) createCollection(w http.ResponseWriter, r *http.Request) {
	var b collectionBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	lib, err := s.store.GetLibrary(r.Context(), b.LibraryID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if lib == nil {
		writeError(w, http.StatusBadRequest, "choose a library")
		return
	}
	if b.Kind != "manual" && b.Kind != "smart" {
		writeError(w, http.StatusBadRequest, `kind is "manual" or "smart"`)
		return
	}
	c := store.Collection{LibraryID: lib.ID, Kind: b.Kind}
	if msg := applyCollectionBody(&c, &b); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.CreateCollection(r.Context(), &c); err != nil {
		s.fail(w, err)
		return
	}
	if c.Kind == "manual" && len(b.ItemIDs) > 0 {
		if err := s.addItems(r, &c, b.ItemIDs); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusCreated, s.collectionDTO(c, nil))
}

// applyCollectionBody copies a body's changes onto a collection, returning a problem if any.
func applyCollectionBody(c *store.Collection, b *collectionBody) string {
	locked := map[string]bool{}
	if b.LockedFields != nil {
		for _, f := range *b.LockedFields {
			switch f {
			case "title", "sort_title", "overview", "poster", "backdrop":
				locked[f] = true
			default:
				return "unknown field in locked_fields: " + f
			}
		}
	} else {
		for _, f := range c.LockedFields {
			locked[f] = true
		}
	}
	if b.Title != nil && strings.TrimSpace(*b.Title) != c.Title {
		c.Title, locked["title"] = strings.TrimSpace(*b.Title), true
	}
	if c.Title == "" || len([]rune(c.Title)) > 100 {
		return "give the collection a name of up to 100 characters"
	}
	if b.SortTitle != nil {
		if v := strings.ToLower(strings.TrimSpace(*b.SortTitle)); v != "" {
			c.SortTitle, locked["sort_title"] = v, true
		} else {
			c.SortTitle, locked["sort_title"] = "", false // follows the title
		}
	} else if !locked["sort_title"] {
		c.SortTitle = ""
	}
	if b.Overview != nil && strings.TrimSpace(*b.Overview) != c.Overview {
		c.Overview, locked["overview"] = strings.TrimSpace(*b.Overview), true
	}
	c.LockedFields = c.LockedFields[:0]
	for _, f := range []string{"title", "sort_title", "overview", "poster", "backdrop"} {
		if locked[f] {
			c.LockedFields = append(c.LockedFields, f)
		}
	}
	if c.Kind == "smart" {
		if b.Filter != nil {
			f := *b.Filter
			f.LibraryID = c.LibraryID
			// The same checks as a custom home row's filter.
			p := prefs{Home: []homeRow{{ID: "custom:x", Title: c.Title, Filter: &f}}}
			if err := p.validate(); err != nil {
				return err.Error()
			}
			raw, _ := json.Marshal(f)
			c.Filter = string(raw)
		}
		if c.Filter == "" {
			return "a smart collection needs a filter"
		}
	}
	return ""
}

// updateCollection: PATCH /api/collections/{id} {"title", "overview", "filter"}.
func (s *Server) updateCollection(w http.ResponseWriter, r *http.Request) {
	c, ok := s.collectionFromPath(w, r)
	if !ok {
		return
	}
	var b collectionBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if msg := applyCollectionBody(c, &b); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.UpdateCollection(r.Context(), c); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.collectionDTO(*c, nil))
}

// deleteCollection: DELETE /api/collections/{id}. Automatic collections are hidden for good.
func (s *Server) deleteCollection(w http.ResponseWriter, r *http.Request) {
	c, ok := s.collectionFromPath(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteCollection(r.Context(), c); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// addItems puts titles in a manual collection after checking they're from its library.
func (s *Server) addItems(r *http.Request, c *store.Collection, ids []string) error {
	for _, id := range ids {
		it, err := s.store.GetItem(r.Context(), id)
		if err != nil {
			return err
		}
		if it == nil || it.LibraryID != c.LibraryID || (it.Kind != "movie" && it.Kind != "series") {
			return errMsg("only movies and shows from the collection's library can be added")
		}
	}
	return s.store.AddToCollection(r.Context(), c.ID, ids)
}

type errMsg string

func (e errMsg) Error() string { return string(e) }

// collectionAddItems: POST /api/collections/{id}/items {"item_ids": [...]} adds titles to a
// manual collection.
func (s *Server) collectionAddItems(w http.ResponseWriter, r *http.Request) {
	c, ok := s.collectionFromPath(w, r)
	if !ok {
		return
	}
	var b struct {
		ItemIDs []string `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || len(b.ItemIDs) == 0 {
		writeError(w, http.StatusBadRequest, `send {"item_ids": [...]}`)
		return
	}
	if c.Kind != "manual" {
		writeError(w, http.StatusBadRequest, "titles are added to manual collections only")
		return
	}
	if err := s.addItems(r, c, b.ItemIDs); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// collectionRemoveItem: DELETE /api/collections/{id}/items/{item} takes a title out of a manual collection.
func (s *Server) collectionRemoveItem(w http.ResponseWriter, r *http.Request) {
	c, ok := s.collectionFromPath(w, r)
	if !ok {
		return
	}
	if err := s.store.RemoveFromCollection(r.Context(), c.ID, r.PathValue("item")); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// orderCollection: PUT /api/collections/{id}/order {"item_ids": [...]} orders a manual collection.
func (s *Server) orderCollection(w http.ResponseWriter, r *http.Request) {
	c, ok := s.collectionFromPath(w, r)
	if !ok {
		return
	}
	var b struct {
		ItemIDs []string `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, `send {"item_ids": [...]}`)
		return
	}
	if err := s.store.OrderCollection(r.Context(), c.ID, b.ItemIDs); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// itemCollections lists the collections a title is in, for its page.
func (s *Server) itemCollections(r *http.Request, it *store.Item) []collectionRef {
	cols, err := s.store.ItemCollections(r.Context(), it, access(r))
	if err != nil {
		return nil
	}
	out := []collectionRef{}
	for _, c := range cols {
		out = append(out, collectionRef{ID: c.ID, Title: c.Title, Kind: c.Kind})
	}
	return out
}

// listCollectionArtwork: GET /api/collections/{id}/artwork lists the posters and backgrounds TMDB
// has for an automatic collection (none for the admin's own), with the ones in use.
func (s *Server) listCollectionArtwork(w http.ResponseWriter, r *http.Request) {
	c, ok := s.collectionFromPath(w, r)
	if !ok {
		return
	}
	posters, backdrops := []metadata.Image{}, []metadata.Image{}
	if c.Kind == "auto" && c.TMDBID != 0 {
		p, b, err := s.scanner.CollectionImages(r.Context(), c.TMDBID)
		if err != nil && !errors.Is(err, scanner.ErrNoProvider) {
			writeError(w, http.StatusBadGateway, "couldn't get artwork from TMDB: "+err.Error())
			return
		}
		if err == nil {
			posters, backdrops = p, b
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"posters": posters, "backdrops": backdrops, "logos": []metadata.Image{},
		"current": map[string]string{"poster": c.PosterPath, "backdrop": c.BackdropPath},
		"chosen":  map[string]bool{"poster": slices.Contains(c.LockedFields, "poster"), "backdrop": slices.Contains(c.LockedFields, "backdrop")},
		"local":   map[string]bool{},
	})
}

// chooseCollectionArtwork: PUT /api/collections/{id}/artwork {"type": "poster", "path": "/abc.jpg"}
// uses a TMDB image; a null path goes back to automatic (TMDB's for automatic collections, none
// otherwise).
func (s *Server) chooseCollectionArtwork(w http.ResponseWriter, r *http.Request) {
	c, ok := s.collectionFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Type string  `json:"type"`
		Path *string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || (body.Type != "poster" && body.Type != "backdrop") ||
		body.Path != nil && !strings.HasPrefix(*body.Path, "/") {
		writeError(w, http.StatusBadRequest, `send {"type": "poster" or "backdrop", "path": "/abc.jpg" or null}`)
		return
	}
	old := c.PosterPath
	if body.Type == "backdrop" {
		old = c.BackdropPath
	}
	path := ""
	if body.Path != nil {
		path = *body.Path
	}
	if err := s.store.SetCollectionArt(r.Context(), c.ID, body.Type, path); err != nil {
		s.fail(w, err)
		return
	}
	s.removeUpload(old)
	if path == "" && c.Kind == "auto" {
		// Back to TMDB's own.
		if info, err := s.scanner.CollectionInfo(r.Context(), c.TMDBID); err == nil && info != nil {
			s.store.SetCollectionInfo(r.Context(), c.ID, info.Overview, info.PosterPath, info.BackdropPath)
		}
	}
	s.getCollection(w, r)
}

// uploadCollectionArtwork: POST /api/collections/{id}/artwork/{type} with an image as the body.
func (s *Server) uploadCollectionArtwork(w http.ResponseWriter, r *http.Request) {
	c, ok := s.collectionFromPath(w, r)
	if !ok {
		return
	}
	typ := r.PathValue("type")
	if typ != "poster" && typ != "backdrop" {
		writeError(w, http.StatusBadRequest, `type is "poster" or "backdrop"`)
		return
	}
	path, ok := s.saveUpload(w, r, "collection-"+c.ID+"-"+typ)
	if !ok {
		return
	}
	old := c.PosterPath
	if typ == "backdrop" {
		old = c.BackdropPath
	}
	if err := s.store.SetCollectionArt(r.Context(), c.ID, typ, path); err != nil {
		s.removeUpload(path)
		s.fail(w, err)
		return
	}
	s.removeUpload(old)
	s.getCollection(w, r)
}

package importer

import (
	"context"
	"fmt"

	"ams/internal/store"
)

// ListItem is a movie, show or episode in a watchlist, playlist or collection on the other server.
type ListItem struct {
	Title string
	Ref   store.ImportRef
}

// List is a playlist or collection: its title and members in order.
type List struct {
	Title string
	Items []ListItem
	// Built from rules on the other server; brought over as a fixed list of what it holds now.
	Smart bool
}

// ListSource is a source that can also bring watchlists, playlists and collections.
type ListSource interface {
	// Watchlist is the account's watchlist (Jellyfin: favourites), or nil when this account's
	// can't be read (Plex: only the signed-in account's).
	Watchlist(ctx context.Context, account Account) ([]ListItem, error)
	// Playlists are the account's own video playlists (nil when they can't be read).
	Playlists(ctx context.Context, account Account) ([]List, error)
	// Collections are the server's collections, shared by everyone.
	Collections(ctx context.Context) ([]List, error)
}

// ListsResult says what came over besides watch state.
type ListsResult struct {
	// Collections made or added to in Arc, and titles placed in them.
	Collections     int `json:"collections"`
	CollectionItems int `json:"collection_items"`
	// Titles in collections that Arc doesn't have.
	NotFound int      `json:"not_found"`
	Missing  []string `json:"missing,omitempty"`
	// Why collections couldn't be read, if they couldn't (everything else still came over).
	Error string `json:"error,omitempty"`
}

// importLists brings an account's watchlist and playlists into its profile.
func importLists(ctx context.Context, st *store.Store, src ListSource, a Account, userID string, res *AccountResult) error {
	watchlist, err := src.Watchlist(ctx, a)
	if err != nil {
		return fmt.Errorf("reading the watchlist: %w", err)
	}
	for _, it := range watchlist {
		id, err := find(ctx, st, it, res)
		if err != nil || id == "" {
			if err != nil {
				return err
			}
			continue
		}
		if err := st.SetWatchlist(ctx, userID, id, true); err != nil {
			return err
		}
		res.Watchlist++
	}
	playlists, err := src.Playlists(ctx, a)
	if err != nil {
		return fmt.Errorf("reading playlists: %w", err)
	}
	for _, pl := range playlists {
		var ids []string
		for _, it := range pl.Items {
			id, err := find(ctx, st, it, res)
			if err != nil {
				return err
			}
			if id != "" {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			continue
		}
		added, err := st.ImportPlaylist(ctx, userID, pl.Title, ids)
		if err != nil {
			return err
		}
		res.Playlists++
		res.PlaylistItems += added
	}
	return nil
}

// importCollections brings the server's collections into the libraries their titles are in.
func importCollections(ctx context.Context, st *store.Store, src ListSource) (ListsResult, error) {
	var res ListsResult
	cols, err := src.Collections(ctx)
	if err != nil {
		return res, fmt.Errorf("reading collections: %w", err)
	}
	for _, c := range cols {
		// Arc's collections belong to a library: one per library its titles are in, in order.
		byLibrary := map[int64][]string{}
		var libraries []int64
		for _, it := range c.Items {
			id, err := st.FindImported(ctx, it.Ref)
			if err != nil {
				return res, err
			}
			if id == "" {
				res.NotFound++
				if len(res.Missing) < 100 {
					res.Missing = append(res.Missing, c.Title+": "+it.Title)
				}
				continue
			}
			item, err := st.GetItem(ctx, id)
			if err != nil || item == nil {
				return res, err
			}
			if _, ok := byLibrary[item.LibraryID]; !ok {
				libraries = append(libraries, item.LibraryID)
			}
			byLibrary[item.LibraryID] = append(byLibrary[item.LibraryID], id)
		}
		for _, lib := range libraries {
			added, err := st.ImportCollection(ctx, lib, c.Title, byLibrary[lib])
			if err != nil {
				return res, err
			}
			res.Collections++
			res.CollectionItems += added
		}
	}
	return res, nil
}

// find matches a list item to Arc's title, counting the ones Arc doesn't have.
func find(ctx context.Context, st *store.Store, it ListItem, res *AccountResult) (string, error) {
	id, err := st.FindImported(ctx, it.Ref)
	if err != nil || id != "" {
		return id, err
	}
	res.ListsNotFound++
	if len(res.Missing) < 100 {
		res.Missing = append(res.Missing, it.Title)
	}
	return "", nil
}

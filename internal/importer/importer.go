// Package importer brings watch state over from Plex and Jellyfin: what each person has watched,
// where they stopped, their ratings and (from Plex) their watch history, plus watchlists,
// playlists and collections (lists.go). It only reads from the other server, never overwrites
// newer progress in Arc, and adds to lists rather than duplicating them when run again.
package importer

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"ams/internal/activity"
	"ams/internal/store"
)

// Account is a person on the other server.
type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// The account signed in with (Plex: the one whose resume points and ratings can be read).
	Owner bool `json:"owner,omitempty"`
}

// Entry is one movie or episode an account has watched, started or rated.
type Entry struct {
	Title string
	Ref   store.ImportRef
	Watch store.ImportedWatch
	// When it was watched (unix seconds), from the other server's history.
	History []int64
}

// Source is a Plex or Jellyfin server being imported from.
type Source interface {
	Name() string
	Accounts(ctx context.Context) ([]Account, error)
	// Entries lists what an account has watched, reporting progress as it goes.
	Entries(ctx context.Context, account Account, progress func(string)) ([]Entry, error)
}

// SourceError lets the job boundary distinguish source reads from database writes
// without logging an upstream error that may include a credential-bearing URL.
type SourceError struct{ Cause error }

func (e *SourceError) Error() string { return e.Cause.Error() }
func (e *SourceError) Unwrap() error { return e.Cause }

// AccountResult says how importing one account went.
type AccountResult struct {
	Account string `json:"account"`
	Profile string `json:"profile"`
	// Titles brought over, ones Arc doesn't have, and viewings added to the history.
	Matched  int `json:"matched"`
	NotFound int `json:"not_found"`
	History  int `json:"history"`
	// Titles added to the profile's watchlist, playlists made or added to and entries added, and
	// list members Arc doesn't have.
	Watchlist     int `json:"watchlist"`
	Playlists     int `json:"playlists"`
	PlaylistItems int `json:"playlist_items"`
	ListsNotFound int `json:"lists_not_found"`
	// Why the watchlist or playlists couldn't be read, if they couldn't (watch state still came over).
	ListsError string `json:"lists_error,omitempty"`
	// Some of the titles Arc doesn't have, so they can be added or looked for.
	Missing []string `json:"missing,omitempty"`
}

// Run imports each account into its Arc profile (accounts not in mapping are skipped), then the
// server's collections.
func Run(ctx context.Context, st *store.Store, src Source, accounts []Account, mapping map[string]string,
	profileNames map[string]string, task *activity.Handle) ([]AccountResult, ListsResult, error) {
	var lists ListsResult
	var out []AccountResult
	var chosen []Account
	for _, a := range accounts {
		if mapping[a.ID] != "" {
			chosen = append(chosen, a)
		}
	}
	for i, a := range chosen {
		userID := mapping[a.ID]
		res := AccountResult{Account: a.Name, Profile: profileNames[userID]}
		label := fmt.Sprintf("%s (%d of %d)", a.Name, i+1, len(chosen))
		task.Detail("Reading " + label)
		entries, err := src.Entries(ctx, a, func(s string) { task.Detail(label + ": " + s) })
		if err != nil {
			return out, lists, fmt.Errorf("%s: %w", a.Name, &SourceError{Cause: err})
		}
		task.Detail("Matching " + label)
		for j, e := range entries {
			if j%50 == 0 {
				task.Progress(j, len(entries))
			}
			id, err := st.FindImported(ctx, e.Ref)
			if err != nil {
				return out, lists, err
			}
			if id == "" {
				res.NotFound++
				if len(res.Missing) < 100 {
					res.Missing = append(res.Missing, e.Title)
				}
				continue
			}
			if err := st.MergeImportedWatch(ctx, userID, id, e.Watch); err != nil {
				return out, lists, err
			}
			res.Matched++
			for _, at := range e.History {
				added, err := st.AddImportedPlay(ctx, userID, id, at)
				if err != nil {
					return out, lists, err
				}
				if added {
					res.History++
				}
			}
		}
		if ls, ok := src.(ListSource); ok {
			task.Detail("Watchlist and playlists for " + label)
			if err := importLists(ctx, st, ls, a, userID, &res); err != nil {
				if ctx.Err() != nil {
					return out, lists, ctx.Err()
				}
				res.ListsError = err.Error()
			}
		}
		out = append(out, res)
	}
	if ls, ok := src.(ListSource); ok && len(chosen) > 0 {
		task.Detail("Collections")
		var err error
		if lists, err = importCollections(ctx, st, ls); err != nil {
			if ctx.Err() != nil {
				return out, lists, ctx.Err()
			}
			lists.Error = err.Error()
		}
	}
	task.Progress(0, 0)
	return out, lists, nil
}

// parseID reads a numeric id ("" or junk is 0).
func parseID(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

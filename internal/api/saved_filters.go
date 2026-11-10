package api

import (
	"ams/internal/store"
	"math"
)

// validate applies the same rules to every saved filter, independent of its row name.
func (f rowFilter) validate() error {
	if err := f.validateExtended(); err != nil {
		return errMsg(err.Error())
	}
	if f.LibraryID <= 0 {
		return errMsg("choose a library for the filter")
	}
	if f.Sort != "" && !store.ItemSorts[f.Sort] {
		return errMsg("unknown sort")
	}
	if f.AddedWithin < 0 || f.MinRuntime < 0 || f.MaxRuntime < 0 || math.IsNaN(f.MinRating) || math.IsInf(f.MinRating, 0) || f.MinRating < 0 || f.MinRating > 10 {
		return errMsg("filter value out of range")
	}
	if f.MaxRuntime > 0 && f.MinRuntime > f.MaxRuntime {
		return errMsg("minimum runtime must not exceed maximum runtime")
	}
	return nil
}

// itemQuery keeps saved collection and playlist evaluation consistent. The library
// supplies identity and kind; caller-supplied access is never stored in the filter.
func (f rowFilter) itemQuery(lib *store.Library, a store.Access, userID string, limit int) store.ItemQuery {
	q := store.ItemQuery{LibraryID: lib.ID, Kind: "movie", Genre: f.Genre, Language: f.Language, Label: f.Label, Decade: f.Decade, Resolution: f.Resolution,
		Unwatched: f.Unwatched, AddedWithinDays: f.AddedWithin, MinRuntime: f.MinRuntime, MaxRuntime: f.MaxRuntime, MinRating: f.MinRating, Sort: f.Sort, Access: a, UserID: userID, Limit: limit}
	f.applyExtended(&q)
	if lib.Kind == "shows" {
		q.Kind = "series"
	}
	return q
}

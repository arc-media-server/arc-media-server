package scanner

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"ams/internal/store"
)

// Unmatch reserves the same library lease as discovery, maintenance and series refresh. Explicit
// Match admission is serialized so a delayed provider response cannot race the metadata reset.
func (s *Scanner) Unmatch(ctx context.Context, it *store.Item) (err error) {
	if it.Kind != "series" {
		return fmt.Errorf("only shows can be unmatched")
	}
	s.manualMatch.Lock()
	defer s.manualMatch.Unlock()
	if s.Matching(it.ID) {
		return ErrLibraryBusy
	}
	work, release, err := s.beginScopedWork(ctx, it.LibraryID, "unmatch", "Removing show metadata")
	if err != nil {
		return err
	}
	defer func() { release(err) }()
	started := time.Now()
	lib, err := s.store.GetLibrary(work, it.LibraryID)
	if err != nil {
		return err
	}
	if lib == nil {
		return fmt.Errorf("library not found")
	}
	title, year, plan, err := s.unmatchPlan(work, lib, it)
	if err != nil {
		return err
	}
	if err = s.store.Unmatch(work, it.ID, title, year, plan); err != nil {
		return err
	}
	s.log.Info("show unmatched", "item_id", it.ID, "library_id", lib.ID, "episodes", len(plan), "took", time.Since(started).Round(time.Millisecond))
	return nil
}

func (s *Scanner) unmatchPlan(ctx context.Context, lib *store.Library, it *store.Item) (string, int, []store.UnmatchEpisode, error) {
	children, err := s.store.Children(ctx, it.ID)
	if err != nil {
		return "", 0, nil, err
	}
	// This title is only a fallback for empty/fileless shows. Provider text is never retained.
	title, year := "Unmatched show", 0
	entries := map[string]store.OrderingEntry{}
	orderings, err := s.store.ListOrderings(ctx, it.ID)
	if err != nil {
		return "", 0, nil, err
	}
	preferred := it.SourceOrdering
	if preferred == "" {
		preferred = lib.SourceOrdering
	}
	for _, o := range orderings {
		if o.Source == preferred || o.Source == "file" {
			es, e := s.store.OrderingEntries(ctx, o.ID)
			if e != nil {
				return "", 0, nil, e
			}
			for _, entry := range es {
				entries[entry.EpisodeID] = entry
			}
			if o.Source == preferred {
				break
			}
		}
	}
	type namedFile struct {
		file store.MediaFile
		name EpisodeName
		show MovieName
	}
	files := map[int64]namedFile{}
	episodeFiles := map[string][]int64{}
	for _, child := range children {
		fs, e := s.store.ItemFiles(ctx, child.ID)
		if e != nil {
			return "", 0, nil, e
		}
		for _, f := range fs {
			for _, root := range lib.Paths {
				rel, e := filepath.Rel(root, f.Path)
				if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					continue
				}
				_, show, name, ok := ParseEpisodePath(root, f.Path)
				if ok {
					files[f.ID] = namedFile{f, name, show}
					episodeFiles[child.ID] = append(episodeFiles[child.ID], f.ID)
				}
				break
			}
		}
	}
	fileIDs := make([]int64, 0, len(files))
	for id := range files {
		fileIDs = append(fileIDs, id)
	}
	slices.Sort(fileIDs)
	if len(fileIDs) > 0 {
		show := files[fileIDs[0]].show
		title, year = show.Title, show.Year
	}
	// A single-file episode is unambiguous even when previous provider entries were removed.
	// For multi-episode files, use the saved file-order positions; never guess identities.
	claimed := map[string]string{}
	plan := make([]store.UnmatchEpisode, 0, len(children))
	for _, child := range children {
		p := store.UnmatchEpisode{ID: child.ID, Title: "Episode", Key: "unmatch:" + child.ID}
		for _, fileID := range episodeFiles[child.ID] {
			f := files[fileID]
			name := f.name
			if name.AirDate != "" {
				p.Title = name.AirDate
				continue
			}
			season := name.Season
			if name.Absolute {
				season = 1
			}
			number := -1
			if old, ok := entries[child.ID]; ok && (old.Season == season || name.Absolute) && slices.Contains(name.Episodes, old.Episode) {
				number = old.Episode
			}
			if number < 0 && len(name.Episodes) == 1 {
				number = name.Episodes[0]
			}
			if number < 0 {
				continue
			}
			key := fileEpisodeKey(it.ID, name.Absolute, season, number)
			p.Title = "Episode " + strconv.Itoa(number)
			p.Season, p.Episode, p.Placed = season, number, true
			if previous := claimed[key]; previous == "" || previous == child.ID {
				p.Key = key
				claimed[key] = child.ID
			}
			break
		}
		plan = append(plan, p)
	}
	return title, year, plan, nil
}

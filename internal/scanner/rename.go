package scanner

import (
	"ams/internal/metadata"
	"ams/internal/store"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func compatibleRenameNames(a, b MovieName) bool {
	return metadata.Normalize(a.Title) != "" && metadata.Normalize(a.Title) == metadata.Normalize(b.Title) &&
		(a.Year == 0 || b.Year == 0 || a.Year == b.Year) && (a.TMDBID == 0 || b.TMDBID == 0 || a.TMDBID == b.TMDBID) &&
		(a.TVDBID == 0 || b.TVDBID == 0 || a.TVDBID == b.TVDBID) && (a.IMDBID == "" || b.IMDBID == "" || a.IMDBID == b.IMDBID)
}
func compatibleRenamePath(lib *store.Library, old, path string) bool {
	if lib.Kind != "shows" {
		return compatibleRenameNames(ParseMovie(old), ParseMovie(path))
	}
	parse := func(path string) (MovieName, EpisodeName, bool) {
		for _, root := range lib.Paths {
			rel, err := filepath.Rel(root, path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			_, name, ep, ok := ParseEpisodePath(root, path)
			if ok {
				return name, ep, true
			}
		}
		return MovieName{}, EpisodeName{}, false
	}
	a, ae, ok := parse(old)
	if !ok {
		return false
	}
	b, be, ok := parse(path)
	return ok && compatibleRenameNames(a, b) && ae.Season == be.Season && ae.Absolute == be.Absolute && ae.AirDate == be.AirDate && slices.Equal(ae.Episodes, be.Episodes)
}
func (s *Scanner) reuseRenamedFile(ctx context.Context, lib *store.Library, path string, info fs.FileInfo) (*store.MediaFile, error) {
	candidates, err := s.store.RenameCandidates(ctx, lib.ID, info.Size(), info.ModTime().Unix())
	if err != nil {
		return nil, err
	}
	if len(candidates) > 32 {
		return nil, nil
	} // Never pick a coincidental fingerprint from a truncated set.
	var candidate *store.MediaFile
	for i := range candidates {
		old := &candidates[i]
		if !compatibleRenamePath(lib, old.Path, path) {
			continue
		}
		if _, err := os.Stat(old.Path); !errors.Is(err, os.ErrNotExist) {
			continue
		}
		if candidate != nil {
			return nil, nil
		} // Ambiguity must not move another title's history/cache.
		candidate = old
	}
	if candidate == nil {
		return nil, nil
	}
	moved, err := s.store.RelocateFile(ctx, *candidate, path)
	if err != nil || !moved {
		return nil, err
	}
	candidate.Path = path
	s.log.Debug("file rename reconciled", "library_id", lib.ID, "file_id", candidate.ID)
	return candidate, nil
}
func (s *Scanner) ensureNamedItem(ctx context.Context, it *store.Item, key string, fileIDs []int64) (string, bool, error) {
	existing, err := s.store.GetItemByKey(ctx, it.LibraryID, key)
	if err != nil {
		return "", false, err
	}
	if existing != nil {
		return existing.ID, false, nil
	}
	owners, err := s.store.OwnersForFiles(ctx, it.LibraryID, it.Kind, fileIDs)
	if err != nil {
		return "", false, err
	}
	if len(owners) == 1 {
		old := owners[0]
		if (old.TMDBID == 0 || it.TMDBID == 0 || old.TMDBID == it.TMDBID) && (old.TVDBID == 0 || it.TVDBID == 0 || old.TVDBID == it.TVDBID) && (old.IMDBID == "" || it.IMDBID == "" || old.IMDBID == it.IMDBID) {
			if err := s.store.RekeyItemWithAlias(ctx, old.ID, key); err != nil {
				return "", false, err
			}
			if err := s.store.AdoptIDs(ctx, old.ID, it.TMDBID, it.TVDBID, it.IMDBID); err != nil {
				return "", false, err
			}
			return old.ID, false, nil
		}
	}
	return s.store.EnsureItem(ctx, it, key)
}

package scanner

import (
	"ams/internal/store"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrScanRootsUnsafe = errors.New("missing-file reconciliation skipped: no readable known file in a previously populated folder; check that the library storage is mounted")

func fileWithinFolder(path, folder string) bool {
	rel, err := filepath.Rel(folder, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Shared with automatic trash: an existing readable regular file, not an empty mount directory.
func readableFileAnchor(ctx context.Context, root string, files []store.MediaFile) bool {
	for _, f := range files {
		if ctx.Err() != nil {
			return false
		}
		if !fileWithinFolder(f.Path, root) {
			continue
		}
		h, err := os.Open(f.Path)
		if err != nil {
			continue
		}
		info, err := h.Stat()
		h.Close()
		if err == nil && info.Mode().IsRegular() && info.Size() == f.Size {
			return true
		}
	}
	return false
}
func (s *Scanner) checkDiscoveryAnchors(ctx context.Context, libraryID, seenAt int64, folders []scanFolder, prior []store.MediaFile) error {
	if len(prior) == 0 {
		return nil
	}
	seen, err := s.store.SeenLibraryFiles(ctx, libraryID, seenAt)
	if err != nil {
		return err
	}
	knownIDs := make(map[int64]bool, len(prior))
	for _, f := range prior {
		knownIDs[f.ID] = true
	}
	anchors := append([]store.MediaFile(nil), prior...)
	for _, f := range seen {
		if knownIDs[f.ID] {
			anchors = append(anchors, f)
		}
	}
	unsafe := 0
	checkedRoots := map[string]bool{}
	for _, folder := range folders {
		if checkedRoots[folder.root] {
			continue
		}
		checkedRoots[folder.root] = true
		known := false
		for _, f := range prior {
			if fileWithinFolder(f.Path, folder.root) {
				known = true
				break
			}
		}
		if known && !readableFileAnchor(ctx, folder.root, anchors) {
			unsafe++
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if unsafe > 0 {
		return fmt.Errorf("%w (%d folders)", ErrScanRootsUnsafe, unsafe)
	}
	return nil
}

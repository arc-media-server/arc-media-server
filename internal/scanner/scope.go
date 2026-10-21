package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ams/internal/store"
)

type scanFolder struct{ root, path string }

// Scope at movie/show-folder boundaries. Parsing still uses the original library root,
// never a season/extras directory as if it were a new show. Root events reconcile fully.
func scanFolders(lib *store.Library, paths []string) ([]scanFolder, error) {
	if len(paths) == 0 {
		var out []scanFolder
		for _, root := range lib.Paths {
			out = append(out, scanFolder{root, root})
		}
		return out, nil
	}
	seen := map[string]bool{}
	var out []scanFolder
	for _, changed := range paths {
		found := false
		for _, root := range lib.Paths {
			rel, err := filepath.Rel(root, filepath.Clean(changed))
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
				continue
			}
			found = true
			parts := strings.Split(rel, string(filepath.Separator))
			if rel == "." {
				return scanFolders(lib, nil)
			}
			if len(parts) == 1 {
				info, err := os.Stat(changed)
				if err != nil || !info.IsDir() {
					return scanFolders(lib, nil)
				}
			}
			path := filepath.Join(root, parts[0])
			if !seen[path] {
				seen[path] = true
				out = append(out, scanFolder{root, path})
				if len(out) > 128 {
					// A large batch is cheaper and safer as one reconciliation; keep
					// the missing-file SQL expression and parameter counts bounded.
					return scanFolders(lib, nil)
				}
			}
			break
		}
		if !found {
			return nil, fmt.Errorf("changed path is outside the library: %s", changed)
		}
	}
	return out, nil
}

func inScanFolders(path string, folders []scanFolder) bool {
	for _, f := range folders {
		if path == f.path || strings.HasPrefix(path, f.path+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

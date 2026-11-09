// Package filecache scopes private assets to a persistent source revision, not a reusable rowid.
package filecache

import (
	"encoding/hex"
	"path/filepath"
	"strconv"
)

func Dir(base string, id int64, key string) (string, bool) {
	if id <= 0 || len(key) != 32 {
		return "", false
	}
	if _, err := hex.DecodeString(key); err != nil {
		return "", false
	}
	return filepath.Join(base, strconv.FormatInt(id, 10), "v2-"+key), true
}

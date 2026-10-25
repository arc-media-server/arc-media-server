package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestBrowseLetterJumpWholeFilteredList(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Jump fixture", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := range 1015 {
		title := fmt.Sprintf("Alpha %04d", i)
		if i >= 510 {
			title = fmt.Sprintf("Sierra %04d", i-510)
		}
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	for _, title := range []string{"The Tango", "Zulu", "007", "Éclair", "東京"} {
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	fid, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: filepath.Join(t.TempDir(), "fixture.mkv")}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetFileItems(ctx, fid, ids, 0); err != nil {
		t.Fatal(err)
	}
	// A hidden title before the requested letter must affect neither its offset nor total.
	if _, err := st.db.ExecContext(ctx, "INSERT INTO item_labels(item_id,label) VALUES(?, 'hidden')", ids[0]); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, letter, first string
		reverse             bool
		offset, total       int
	}{
		{"beyond first 500", "s", "Sierra 0000", false, 510, 1019},
		{"absent letter chooses next", "r", "Sierra 0000", false, 510, 1019},
		{"reverse starts at end of letter", "s", "Sierra 0504", true, 4, 1019},
		{"reverse missing letter chooses earlier", "r", "Alpha 0509", true, 509, 1019},
		{"article uses sort title", "t", "The Tango", false, 1015, 1019},
		{"empty library permission", "s", "", false, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := ItemQuery{LibraryID: lib, Kind: "movie", Jump: tc.letter, Reverse: tc.reverse, Limit: 500, Access: Access{ExcludeLabels: []string{"hidden"}}}
			if tc.total == 0 {
				q.Access.Libraries = []int64{}
			}
			items, total, offset, err := st.ListItemsPage(ctx, q)
			if err != nil || total != tc.total || offset != tc.offset {
				t.Fatalf("total=%d offset=%d err=%v", total, offset, err)
			}
			if tc.first == "" {
				if len(items) != 0 {
					t.Fatal("restricted items returned")
				}
				return
			}
			if len(items) == 0 || items[0].Title != tc.first {
				t.Fatalf("wrong first item: %+v", items)
			}
			if tc.name == "beyond first 500" {
				if len(items) != 500 {
					t.Fatalf("page length %d", len(items))
				}
				q.Jump, q.Offset = "", offset+len(items)
				next, n, off, err := st.ListItemsPage(ctx, q)
				if err != nil || n != total || off != 1010 || len(next) != 9 || next[0].Title != "Sierra 0500" || next[len(next)-1].Title != "東京" {
					t.Fatalf("continued page: total=%d offset=%d len=%d err=%v", n, off, len(next), err)
				}
			}
		})
	}
	// Ordinary browse still starts with numeric titles and includes non-Latin titles.
	all, total, err := st.ListItems(ctx, ItemQuery{LibraryID: lib, Limit: 1})
	if err != nil || total != 1020 || all[0].Title != "007" {
		t.Fatalf("normal browse changed: %+v %d %v", all, total, err)
	}
	filtered, n, off, err := st.ListItemsPage(ctx, ItemQuery{LibraryID: lib, Jump: "s", Search: "Tango", Access: Access{ExcludeLabels: []string{"hidden"}}})
	if err != nil || n != 1 || off != 0 || len(filtered) != 1 || filtered[0].Title != "The Tango" {
		t.Fatalf("jump dropped search: %+v %d %d %v", filtered, n, off, err)
	}
	for _, q := range []ItemQuery{{Jump: "ss"}, {Jump: "s", Sort: "rating"}, {Jump: "'"}} {
		if _, _, _, err := st.ListItemsPage(ctx, q); err == nil {
			t.Fatalf("accepted invalid jump %+v", q)
		}
	}
}

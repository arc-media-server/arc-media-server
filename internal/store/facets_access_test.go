package store

import "testing"

func TestFacetStreamPassKeepsOwnFilesAndEpisodeRestrictions(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	lib, err := st.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	series, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Show"}, "show")
	if err != nil {
		t.Fatal(err)
	}
	pg := "PG"
	if err = st.EditItem(ctx, series, ItemEdit{ContentRating: &pg}); err != nil {
		t.Fatal(err)
	}
	add := func(key, kind, codec string, missing, blocked bool) {
		t.Helper()
		id := series
		if key != "own" {
			var err error
			id, _, err = st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: series, Title: key}, key)
			if err != nil {
				t.Fatal(err)
			}
		}
		f, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/fixture/" + key, Streams: []Stream{{Kind: kind, Codec: codec, Channels: 6}}}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = st.SetFileItems(ctx, f, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		if missing {
			if _, err = st.db.ExecContext(ctx, `UPDATE media_files SET missing_since=1 WHERE id=?`, f); err != nil {
				t.Fatal(err)
			}
		}
		if blocked {
			labels := []string{"blocked"}
			if err = st.EditItem(ctx, id, ItemEdit{Labels: &labels}); err != nil {
				t.Fatal(err)
			}
		}
	}
	add("own", "audio", "flac", false, false)
	add("visible", "video", "h264", false, false)
	add("duplicate", "video", "h264", false, false)
	add("hidden", "video", "hevc", false, true)
	add("offline", "video", "av1", true, false)
	f, err := st.Facets(ctx, lib, "series", Access{MaxAge: 12, ExcludeLabels: []string{"blocked"}})
	if err != nil {
		t.Fatal(err)
	}
	// Episode label access inherits the parent, matching the existing listing rules.
	if len(f.VideoCodecs) != 2 || f.VideoCodecs[0] != (Facet{"h264", 1}) || f.VideoCodecs[1] != (Facet{"hevc", 1}) {
		t.Fatal("child visibility/counting", f.VideoCodecs)
	}
	if len(f.AudioCodecs) != 1 || f.AudioCodecs[0] != (Facet{"flac", 1}) {
		t.Fatal("own files lost", f.AudioCodecs)
	}
	if len(f.AudioChannels) != 1 || f.AudioChannels[0] != (Facet{"6", 1}) {
		t.Fatal(f.AudioChannels)
	}
	labels := []string{"blocked"}
	if err = st.EditItem(ctx, series, ItemEdit{Labels: &labels}); err != nil {
		t.Fatal(err)
	}
	f, err = st.Facets(ctx, lib, "series", Access{ExcludeLabels: []string{"blocked"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.VideoCodecs)+len(f.AudioCodecs)+len(f.AudioChannels)+len(f.DynamicRanges) != 0 {
		t.Fatal("blocked parent leaked facets", f)
	}
	f, err = st.Facets(ctx, lib, "series", Access{Libraries: []int64{lib + 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.VideoCodecs)+len(f.AudioCodecs)+len(f.AudioChannels)+len(f.DynamicRanges) != 0 {
		t.Fatal("denied library leaked facets", f)
	}
}

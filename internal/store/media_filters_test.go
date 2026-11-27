package store

import "testing"

func TestMediaFiltersSameFileStreamAndSeriesAccess(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	lib, _ := st.SyncLibrary(ctx, "Media", "shows", "tvdb_aired", nil)
	series, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Show"}, "show")
	ep, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, ParentID: series, Kind: "episode", Title: "Episode"}, "ep")
	save := func(path string, streams []Stream) {
		t.Helper()
		for i := range streams {
			streams[i].Index = i
		}
		file, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: path, Streams: streams}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = st.SetFileItems(ctx, file, []string{ep}, 0); err != nil {
			t.Fatal(err)
		}
	}
	save("/fixture/a.mkv", []Stream{{Kind: "video", Codec: "hevc", BitDepth: 10, DolbyVision: true, ColorTransfer: "smpte2084"}, {Kind: "audio", Codec: "truehd", Channels: 2}, {Kind: "audio", Codec: "aac", Channels: 8}})
	save("/fixture/b.mkv", []Stream{{Kind: "video", Codec: "h264", BitDepth: 8}, {Kind: "audio", Codec: "truehd", Channels: 8}})
	check := func(q ItemQuery, want int) {
		t.Helper()
		q.LibraryID, q.Kind = lib, "series"
		_, n, err := st.ListItems(ctx, q)
		if err != nil || n != want {
			t.Fatalf("query %+v: n=%d want %d err=%v", q, n, want, err)
		}
	}
	check(ItemQuery{VideoCodec: "hevc", AudioCodec: "truehd", AudioChannels: 8}, 0)
	check(ItemQuery{VideoCodec: "hevc", AudioCodec: "truehd", AudioChannels: 2, DynamicRange: "dolby_vision"}, 1)
	check(ItemQuery{DynamicRange: "hdr10"}, 0)
	check(ItemQuery{VideoCodec: "H264", DynamicRange: "sdr", AudioChannels: 8}, 1)
	check(ItemQuery{DynamicRange: "unknown"}, 0)
	check(ItemQuery{DynamicRange: "dolby_vision", Access: Access{Libraries: []int64{lib + 1}}}, 0)
	// Existing profile label restrictions apply to a show and its inherited episode visibility.
	_, err := st.db.ExecContext(ctx, `INSERT INTO item_labels(item_id,label) VALUES (?,?)`, series, "blocked")
	if err != nil {
		t.Fatal(err)
	}
	check(ItemQuery{VideoCodec: "hevc", Access: Access{ExcludeLabels: []string{"blocked"}}}, 0)
}

func TestMediaFacetsCountTitlesAndRespectAccess(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	lib, _ := st.SyncLibrary(ctx, "Films", "movies", "tmdb_aired", nil)
	add := func(title, rating string, streams []Stream) string {
		t.Helper()
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		for i := range streams {
			streams[i].Index = i
		}
		file, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/fixture/" + title + ".mkv", Streams: streams}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = st.SetFileItems(ctx, file, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		if rating != "" {
			if err = st.EditItem(ctx, id, ItemEdit{ContentRating: &rating}); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	// Duplicate tracks count each title once per facet, and a title can cover two resolutions.
	duneVideo := Stream{Kind: "video", Codec: "HEVC", Width: 3840, Height: 1608, BitDepth: 10, DolbyVision: true}
	duneHD := duneVideo
	duneHD.Width, duneHD.Height = 1920, 816
	add("Dune", "PG", []Stream{duneVideo, duneVideo, duneHD, {Kind: "audio", Codec: "truehd", Channels: 8}, {Kind: "audio", Codec: "truehd", Channels: 2}})
	heat := add("Heat", "PG", []Stream{{Kind: "video", Codec: "h264", Width: 1920, Height: 816, BitDepth: 8}, {Kind: "audio", Codec: "ac3", Channels: 6}})
	add("Saw", "18", []Stream{{Kind: "video", Codec: "hevc", Height: 720, ColorTransfer: "smpte2084"}, {Kind: "audio", Codec: "eac3", Channels: 6}})
	add("Raw", "PG", []Stream{{Kind: "video", Codec: "av1", Width: 3840}}) // no measured height
	offline, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/fixture/offline-heat.mkv",
		Streams: []Stream{{Kind: "video", Codec: "h264", Height: 480}}}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.SetFileItems(ctx, offline, []string{heat}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err = st.db.ExecContext(ctx, `UPDATE media_files SET missing_since=1 WHERE id=?`, offline); err != nil {
		t.Fatal(err)
	}
	facets := func(acc Access) LibraryFacets {
		t.Helper()
		f, err := st.Facets(ctx, lib, "movie", acc)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	same := func(name string, got []Facet, want ...Facet) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: %+v, want %+v", name, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%s: %+v, want %+v", name, got, want)
			}
		}
	}
	f := facets(Access{})
	same("video codecs", f.VideoCodecs, Facet{"hevc", 2}, Facet{"av1", 1}, Facet{"h264", 1})
	same("audio codecs", f.AudioCodecs, Facet{"ac3", 1}, Facet{"eac3", 1}, Facet{"truehd", 1})
	same("resolutions ordered by height", f.Resolutions, Facet{"4k", 1}, Facet{"1080", 2}, Facet{"720", 1})
	same("channels", f.AudioChannels, Facet{"6", 2}, Facet{"2", 1}, Facet{"8", 1})
	same("dynamic range", f.DynamicRanges, Facet{"dolby_vision", 1}, Facet{"hdr10", 1}, Facet{"sdr", 1}, Facet{"unknown", 1})
	// Each value finds the titles it counted.
	for _, c := range []struct {
		q    ItemQuery
		want int
	}{{ItemQuery{VideoCodec: "hevc"}, 2}, {ItemQuery{AudioChannels: 6}, 2}, {ItemQuery{DynamicRange: "unknown"}, 1}} {
		c.q.LibraryID, c.q.Kind = lib, "movie"
		if _, n, err := st.ListItems(ctx, c.q); err != nil || n != c.want {
			t.Fatalf("%+v: %d, want %d (%v)", c.q, n, c.want, err)
		}
	}
	// A profile limited by age doesn't see the restricted title's details counted.
	f = facets(Access{MaxAge: 12})
	same("restricted video codecs", f.VideoCodecs, Facet{"av1", 1}, Facet{"h264", 1}, Facet{"hevc", 1})
	same("restricted resolutions", f.Resolutions, Facet{"4k", 1}, Facet{"1080", 2})
	same("restricted ranges", f.DynamicRanges, Facet{"dolby_vision", 1}, Facet{"sdr", 1}, Facet{"unknown", 1})
}

func TestMediaFacetsCountShowsOnceThroughEpisodes(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	lib, _ := st.SyncLibrary(ctx, "Shows", "shows", "tvdb_aired", nil)
	series, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Show"}, "show")
	for i, codec := range []string{"hevc", "hevc", "h264"} {
		key := string(rune('a' + i))
		ep, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, ParentID: series, Kind: "episode", Title: key}, "ep-"+key)
		file, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/fixture/" + key + ".mkv", Streams: []Stream{{Kind: "video", Codec: codec}}}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = st.SetFileItems(ctx, file, []string{ep}, 0); err != nil {
			t.Fatal(err)
		}
	}
	f, err := st.Facets(ctx, lib, "series", Access{})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.VideoCodecs) != 2 || f.VideoCodecs[0] != (Facet{"h264", 1}) || f.VideoCodecs[1] != (Facet{"hevc", 1}) {
		t.Fatalf("a show counts once per codec: %+v", f.VideoCodecs)
	}
}

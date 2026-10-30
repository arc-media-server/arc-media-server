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

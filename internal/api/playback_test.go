package api

import (
	"slices"
	"testing"

	"ams/internal/store"
)

func TestDecide(t *testing.T) {
	chrome := clientCaps{Containers: []string{"mp4", "webm"}, Video: []string{"h264", "vp9", "av1"}, Audio: []string{"aac", "mp3", "opus", "flac"}}
	file := func(container string, streams ...store.Stream) *store.MediaFile {
		for i := range streams {
			streams[i].Index = i
		}
		return &store.MediaFile{Container: container, Streams: streams}
	}
	h264 := store.Stream{Kind: "video", Codec: "h264", BitDepth: 8, Height: 1080}
	aac := store.Stream{Kind: "audio", Codec: "aac", Channels: 2, IsDefault: true}

	for _, tc := range []struct {
		name      string
		f         *store.MediaFile
		audio     int // stream index, -1 for none
		want      string
		copyAudio bool
		reasons   []string
	}{
		{"mp4 h264 aac", file("mp4", h264, aac), 1, "direct", true, nil},
		{"mov counts as mp4", file("mov", h264, aac), 1, "direct", true, nil},
		{"mkv h264 aac", file("matroska", h264, aac), 1, "remux", true, []string{"MKV file"}},
		{"mp4 with ac3", file("mp4", h264, store.Stream{Kind: "audio", Codec: "ac3", Channels: 6, IsDefault: true}), 1, "remux", false,
			[]string{"Dolby Digital audio"}},
		{"5.1 aac is converted, not copied", file("matroska", h264, store.Stream{Kind: "audio", Codec: "aac", Channels: 6}), 1, "remux", false,
			[]string{"MKV file"}},
		{"second audio track", file("mp4", h264, aac, store.Stream{Kind: "audio", Codec: "aac", Channels: 2}), 2, "remux", true,
			[]string{"another audio track chosen"}},
		{"hevc", file("matroska", store.Stream{Kind: "video", Codec: "hevc", BitDepth: 10}, aac), 1, "transcode", true,
			[]string{"HEVC video", "MKV file"}},
		{"10-bit h264", file("mp4", store.Stream{Kind: "video", Codec: "h264", BitDepth: 10}, aac), 1, "transcode", true,
			[]string{"10-bit H.264 video"}},
		{"no audio", file("mp4", h264), -1, "direct", false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var audio *store.Stream
			if tc.audio >= 0 {
				audio = &tc.f.Streams[tc.audio]
			}
			d := decide(tc.f, videoStream(tc.f), audio, chrome)
			if d.Method != tc.want || d.CopyAudio != tc.copyAudio || !slices.Equal(d.Reasons, tc.reasons) {
				t.Errorf("got %s copyAudio=%v %q, want %s copyAudio=%v %q", d.Method, d.CopyAudio, d.Reasons, tc.want, tc.copyAudio, tc.reasons)
			}
		})
	}

	// A browser that plays HEVC and MKV (e.g. Edge with the HEVC extension) plays it directly.
	edge := chrome
	edge.Video = append(edge.Video, "hevc")
	edge.Containers = append(edge.Containers, "mkv")
	f := file("matroska", store.Stream{Kind: "video", Codec: "hevc", BitDepth: 10}, aac)
	if d := decide(f, videoStream(f), &f.Streams[1], edge); d.Method != "direct" {
		t.Errorf("hevc on edge: %+v", d)
	}
}

func TestBestVersion(t *testing.T) {
	chrome := clientCaps{Containers: []string{"mp4"}, Video: []string{"h264"}, Audio: []string{"aac"}, MaxHeight: 1080}
	file := func(id int64, edition, codec string, height int) store.MediaFile {
		return store.MediaFile{ID: id, Container: "matroska", Edition: edition, Streams: []store.Stream{
			{Index: 0, Kind: "video", Codec: codec, BitDepth: 8, Width: height * 16 / 9, Height: height},
			{Index: 1, Kind: "audio", Codec: "aac", Channels: 2, IsDefault: true}}}
	}
	files := []store.MediaFile{
		file(1, "", "hevc", 2160), // needs converting in Chrome
		file(2, "", "h264", 1080), // repackaged only
		file(3, "", "h264", 720),
		file(4, "Director's Cut", "h264", 1080),
		{ID: 5, Missing: true},
	}
	pick := func(edition string, caps clientCaps) int64 { return bestVersion(files, edition, caps).ID }
	if got := pick("", chrome); got != 2 {
		t.Errorf("chrome: picked %d, want the 1080p H.264", got)
	}
	// A 4K screen that plays HEVC gets the 4K file.
	tv := clientCaps{Containers: []string{"mp4", "mkv"}, Video: []string{"h264", "hevc"}, Audio: []string{"aac"}, MaxHeight: 2160}
	if got := pick("", tv); got != 1 {
		t.Errorf("4K HEVC screen: picked %d", got)
	}
	// A small screen gets the smallest that fills it, not 4K.
	phone := chrome
	phone.MaxHeight = 720
	if got := pick("", phone); got != 3 {
		t.Errorf("720p screen: picked %d", got)
	}
	if got := pick("director's cut", chrome); got != 4 {
		t.Errorf("edition: picked %d", got)
	}
	if got := len(versions(files)); got != 4 {
		t.Errorf("versions lists %d (missing files are left out)", got)
	}
}

func TestPreferredAudio(t *testing.T) {
	f := &store.MediaFile{Streams: []store.Stream{
		{Index: 0, Kind: "video"},
		{Index: 1, Kind: "audio", Language: "eng", IsDefault: true},
		{Index: 2, Kind: "audio", Language: "jpn"},
		{Index: 3, Kind: "audio", Language: "jpn", Title: "Commentary"},
	}}
	if a := preferredAudio(f, "jpn"); a == nil || a.Index != 2 {
		t.Errorf("jpn: %+v", a)
	}
	if a := preferredAudio(f, "ja"); a == nil || a.Index != 2 {
		t.Errorf("2-letter code: %+v", a)
	}
	if a := preferredAudio(f, "fre"); a != nil {
		t.Errorf("a language the file hasn't: %+v", a)
	}
}

func TestQualitySteps(t *testing.T) {
	for kbps, want := range map[int]string{20000: "20 Mbps 1080p", 8000: "8 Mbps 1080p", 4000: "4 Mbps 720p", 1500: "1.5 Mbps 480p", 720: "720 kbps 360p"} {
		if got := qualityName(kbps); got != want {
			t.Errorf("%d: got %q, want %q", kbps, got, want)
		}
	}
}

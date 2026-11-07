package api

import (
	"net/http/httptest"
	"slices"
	"testing"

	"ams/internal/settings"
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
		{"audio explicitly disabled", file("mp4", h264, aac), -1, "remux", false, []string{"audio disabled"}},
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

	// A player that doesn't list H.264 at all is told so, not "10-bit H.264" for every file.
	noH264 := chrome
	noH264.Video = []string{"vp9"}
	plain := file("mp4", h264, aac)
	if d := decide(plain, videoStream(plain), &plain.Streams[1], noH264); d.Method != "transcode" || !slices.Equal(d.Reasons, []string{"H.264 video"}) {
		t.Errorf("player without h264: got %s %q, want transcode [H.264 video]", d.Method, d.Reasons)
	}

	// A retry after the player failed says so, rather than blaming the file's video.
	retry := chrome
	retry.Video, retry.Containers, retry.FailedMethod = nil, nil, "remux"
	if d := decide(plain, videoStream(plain), &plain.Streams[1], retry); d.Method != "transcode" || !slices.Equal(d.Reasons, []string{"the player couldn't play the repackaged stream"}) {
		t.Errorf("retry after remux: got %s %q", d.Method, d.Reasons)
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

func TestRemoteLimit(t *testing.T) {
	s := &Server{playing: newNowPlaying(), settings: func() settings.Settings {
		return settings.Settings{RemoteKbps: 8000, UploadKbps: 10000}
	}}
	home := httptest.NewRequest("POST", "/", nil)
	home.RemoteAddr = "192.168.1.20:5000"
	if got := s.remoteLimit(home, "x"); got != 0 {
		t.Errorf("at home = %d, want no limit", got)
	}
	away := httptest.NewRequest("POST", "/", nil)
	away.RemoteAddr = "203.0.113.5:5000"
	// One stream: the smaller of the 8 Mbps limit and 80% of the 10 Mbps upload.
	if got := s.remoteLimit(away, "x"); got != 8000 {
		t.Errorf("one remote stream = %d, want 8000", got)
	}
	// Another remote stream playing: the upload is shared.
	other := httptest.NewRequest("POST", "/", nil)
	other.RemoteAddr = "198.51.100.7:5000"
	s.playing.touch(other, "y", "")
	s.playing.mu.Lock()
	for _, p := range s.playing.byKey {
		p.address = "198.51.100.7"
	}
	s.playing.mu.Unlock()
	if got := s.remoteLimit(away, "x"); got != 4000 {
		t.Errorf("two remote streams = %d, want 4000", got)
	}
}

// Richer source labels must not change the colour-transfer contract used for tone mapping.
func TestVersionSourceFormatsDoNotChangePlaybackHDR(t *testing.T) {
	f := store.MediaFile{ID: 1, Streams: []store.Stream{{Kind: "video", Codec: "hevc", ColorTransfer: "smpte2084", DolbyVision: true, HDR10Plus: true}}}
	v := versions([]store.MediaFile{f})
	if len(v) != 1 || v[0].HDR != "HDR10" || !slices.Equal(v[0].SourceFormats, []string{"Dolby Vision", "HDR10+"}) {
		t.Fatalf("%+v", v)
	}
	f.Streams[0].DolbyVision = false
	f.Streams[0].HDR10Plus = false
	f.Streams[0].ColorTransfer = "arib-std-b67"
	v = versions([]store.MediaFile{f})
	if v[0].HDR != "HLG" || !slices.Equal(v[0].SourceFormats, []string{"HLG"}) {
		t.Fatalf("%+v", v)
	}
}

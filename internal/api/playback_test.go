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

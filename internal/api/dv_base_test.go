package api

import (
	"ams/internal/store"
	"encoding/json"
	"testing"
)

func TestDVBaseLayerFallback(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		profile, compat       int
		hdr                   []string
		wantTone, unsupported bool
	}{
		{"P7 HDR10", 7, 6, []string{"hdr10"}, false, false},
		{"P81 HDR10", 8, 1, []string{"hdr10"}, false, false},
		{"P81 SDR", 8, 1, []string{}, true, false},
		{"P84 HLG", 8, 4, []string{"hlg"}, false, false},
		{"P84 SDR", 8, 4, []string{}, true, false},
		{"P82 SDR", 8, 2, []string{}, false, false},
		{"P5", 5, 0, []string{}, true, true},
		{"P8 unknown base", 8, 0, []string{"hdr10"}, true, true},
		{"P8 invalid compatibility", 8, 6, []string{"hdr10"}, true, true},
		{"P7 invalid compatibility", 7, 4, []string{"hlg"}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(map[string]any{"kind": "video", "codec": "hevc", "bit_depth": 10, "dolby_vision": true, "dolby_vision_profile": tc.profile, "dv_bl_signal_compatibility_id": tc.compat})
			var video store.Stream
			if err := json.Unmarshal(b, &video); err != nil {
				t.Fatal(err)
			}
			f := &store.MediaFile{Container: "matroska", Streams: []store.Stream{video}}
			d := decide(f, &f.Streams[0], nil, clientCaps{Containers: []string{"mkv", "mp4"}, Video: []string{"hevc", "h264"}, MaxBitDepth: 10, HDRFormats: tc.hdr})
			if d.NeedsToneMap != tc.wantTone || d.UnsupportedDV != tc.unsupported {
				t.Fatalf("unsafe/needless refusal: %+v", d)
			}
		})
	}
}

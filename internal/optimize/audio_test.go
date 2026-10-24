package optimize

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"ams/internal/probe"
	"ams/internal/store"
)

func TestOptimizedUnknownMeasurementIsConservative(t *testing.T) {
	for _, mode := range []string{"probe unavailable", "probe failed", "no measured duration"} {
		t.Run(mode, func(t *testing.T) {
			x := conversionTest(t)
			helperConverter(t, x)
			if mode != "probe unavailable" {
				x.m.Probe = func(context.Context, string) (*probe.Result, error) {
					if mode == "probe failed" {
						return nil, errors.New("measurement fixture failure")
					}
					return &probe.Result{Bitrate: 1234}, nil
				}
			}
			if err := os.WriteFile(filepath.Join(x.root, "release"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			if err := x.m.convert(t.Context(), x.queue(t, 720), nil); err != nil {
				t.Fatal(err)
			}
			copy := x.copy(t, 720)
			if !Usable(&copy, x.f) || !copy.WithinBitrate(0) || copy.WithinBitrate(2000) || copy.Bitrate != 0 || copy.DurationMS != 0 {
				t.Fatalf("unknown measurement accepted for a limited stream: %+v", copy)
			}
		})
	}
}

func TestRealOptimizedAudioProvenance(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe not installed")
	}
	for _, tc := range []struct {
		name                string
		defaultTrack, index int
		language            string
		hz                  float64
	}{
		{"default English", 0, 1, "eng", 440},
		{"default French", 1, 2, "fra", 880},
		{"first audio without default", -1, 1, "eng", 440},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := conversionTest(t)
			args := []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=duration=3:size=160x90:rate=10",
				"-f", "lavfi", "-i", "sine=duration=3:frequency=440", "-f", "lavfi", "-i", "sine=duration=3:frequency=880",
				"-map", "0:v", "-map", "1:a", "-map", "2:a", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac",
				"-metadata:s:a:0", "language=eng", "-metadata:s:a:1", "language=fra", "-disposition:a:0", "0", "-disposition:a:1", "0"}
			if tc.defaultTrack == 0 {
				args = append(args, "-disposition:a:0", "default")
			}
			if tc.defaultTrack == 1 {
				args = append(args, "-disposition:a:1", "default")
			}
			out, err := exec.Command(ffmpeg, append(args, x.f.Path)...).CombinedOutput()
			if err != nil {
				t.Fatalf("generate: %v %s", err, out)
			}
			x.f.Streams = append(x.f.Streams,
				store.Stream{Index: 1, Kind: "audio", Codec: "aac", Channels: 1, Language: "eng", IsDefault: tc.defaultTrack == 0},
				store.Stream{Index: 2, Kind: "audio", Codec: "aac", Channels: 1, Language: "fra", IsDefault: tc.defaultTrack == 1})
			x.save(t)
			x.m.FFmpeg = func() string { return ffmpeg }
			x.m.Probe = probe.New(ffprobe).Probe
			if err := x.m.convert(t.Context(), x.queue(t, 720), nil); err != nil {
				t.Fatal(err)
			}
			copy := x.copy(t, 720)
			if copy.DurationMS < 2900 || copy.DurationMS > 3300 || copy.Bitrate <= 0 {
				t.Fatalf("missing measured output metadata: %+v", copy)
			}
			if copy.AudioStream != tc.index || copy.AudioLanguage != tc.language || !copy.MatchesAudio(&x.f.Streams[tc.index]) || !Usable(&copy, x.f) {
				t.Fatalf("wrong provenance: %+v", copy)
			}
			other := 3 - tc.index
			if copy.MatchesAudio(&x.f.Streams[other]) || copy.MatchesAudio(nil) {
				t.Fatal("mismatching audio accepted")
			}
			out, err = exec.Command(ffprobe, "-v", "error", "-select_streams", "a", "-show_streams", "-of", "json", copy.Path).CombinedOutput()
			if err != nil {
				t.Fatalf("probe: %v %s", err, out)
			}
			var actual struct {
				Streams []struct {
					Index       int
					Codec       string `json:"codec_name"`
					Channels    int
					Tags        struct{ Language string }
					Disposition struct{ Default int }
				}
			}
			if err := json.Unmarshal(out, &actual); err != nil {
				t.Fatal(err)
			}
			if len(actual.Streams) != 1 {
				t.Fatalf("audio count: %s", out)
			}
			a := actual.Streams[0]
			if a.Index != 1 || a.Codec != "aac" || a.Channels != 2 || a.Tags.Language != tc.language || a.Disposition.Default != 1 {
				t.Fatalf("actual audio: %s", out)
			}
			// Decode actual samples: language tags alone cannot prove which source was mapped.
			raw, err := exec.Command(ffmpeg, "-v", "error", "-i", copy.Path, "-map", "0:a:0", "-ss", "0.2", "-t", "1", "-ac", "1", "-ar", "8000", "-f", "f32le", "pipe:1").Output()
			if err != nil {
				t.Fatal(err)
			}
			crossings := 0
			last := float32(0)
			for i := 0; i+4 <= len(raw); i += 4 {
				sample := math.Float32frombits(binary.LittleEndian.Uint32(raw[i : i+4]))
				if sample > 0 && last <= 0 {
					crossings++
				}
				last = sample
			}
			hz := float64(crossings) / (float64(len(raw)/4) / 8000)
			if math.Abs(hz-tc.hz) > 5 {
				t.Fatalf("decoded %.1f Hz want %.1f", hz, tc.hz)
			}
			t.Logf("source track %d; output AAC stereo/%s; decoded %.1f Hz", copy.AudioStream, copy.AudioLanguage, hz)
		})
	}
}

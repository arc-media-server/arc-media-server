package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Re-exec this test binary as a portable fake ffprobe; TestMain handles its arguments
// before testing parses flags. No shell or installed media tools are required.
func TestMain(m *testing.M) {
	if mode := os.Getenv("ARC_TEST_FORMAT_PROBE"); mode != "" {
		if strings.Contains(strings.Join(os.Args, " "), "-show_frames") {
			if mode == "wait" {
				time.Sleep(10 * time.Second)
				os.Exit(1)
			}
			if mode == "fail" {
				os.Exit(1)
			}
			idx := ""
			for i, a := range os.Args {
				if a == "-select_streams" && i+1 < len(os.Args) {
					idx = os.Args[i+1]
				}
			}
			if !strings.Contains(strings.Join(os.Args, " "), "%+#64") {
				os.Exit(3)
			}
			if idx == "4" {
				fmt.Print(`{"frames":[{"stream_index":4,"side_data_list":[{"side_data_type":"Dolby Vision RPU Data"},{"side_data_type":"HDR Dynamic Metadata SMPTE2094-40 (HDR10+)"}]}]}`)
			} else {
				fmt.Print(`{"frames":[{"stream_index":0}]}`)
			}
		} else {
			fmt.Print(`{"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","color_transfer":"smpte2084","width":720,"height":480,"sample_aspect_ratio":"32:27"},{"index":2,"codec_type":"audio","codec_name":"truehd","profile":"Dolby TrueHD + Dolby Atmos","channels":8,"channel_layout":"7.1"},{"index":4,"codec_type":"video","codec_name":"av1","width":1920,"height":800,"display_aspect_ratio":"12:5"}]}`)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestFormatSideData(t *testing.T) {
	for _, tt := range []struct {
		name, data string
		dv, plus   bool
		profile    int
	}{
		{"declared DV", `[{"side_data_type":"DOVI configuration record","dv_profile":8,"rpu_present_flag":1}]`, true, false, 8},
		{"no RPU", `[{"side_data_type":"DOVI configuration record","dv_profile":8,"rpu_present_flag":0}]`, false, false, 0},
		{"hybrid", `[{"side_data_type":"Dolby Vision Metadata","vdr_rpu_profile":1},{"side_data_type":"HDR Dynamic Metadata SMPTE2094-40 (HDR10+)"}]`, true, true, 0},
		{"alternate stream name", `[{"side_data_type":"HDR10+ Dynamic Metadata (SMPTE 2094-40)"}]`, false, true, 0},
		{"static is not dynamic", `[{"side_data_type":"Mastering display metadata"},{"side_data_type":"Content light level metadata"}]`, false, false, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var data []formatSideData
			if err := json.Unmarshal([]byte(tt.data), &data); err != nil {
				t.Fatal(err)
			}
			var s Stream
			applyFormatSideData(&s, data)
			if s.DolbyVision != tt.dv || s.HDR10Plus != tt.plus || s.DolbyVisionProfile != tt.profile {
				t.Fatalf("%+v", s)
			}
		})
	}
}

func TestProbeFormatsKeepsStreamsAndAudioProfile(t *testing.T) {
	t.Setenv("ARC_TEST_FORMAT_PROBE", "sample")
	r, err := New(os.Args[0]).Probe(context.Background(), "pretend.DV.Atmos.mkv")
	if err != nil {
		t.Fatal(err)
	}
	a, b, c := r.Streams[0], r.Streams[1], r.Streams[2]
	if a.DolbyVision || a.HDR10Plus || a.HDRProbeStatus != "sampled" {
		t.Fatalf("wrong first video: %+v", a)
	}
	if b.Profile != "Dolby TrueHD + Dolby Atmos" || b.ChannelLayout != "7.1" {
		t.Fatalf("lost audio: %+v", b)
	}
	if !c.DolbyVision || !c.HDR10Plus || c.HDRProbeStatus != "sampled" {
		t.Fatalf("wrong second video: %+v", c)
	}
}

func TestOptionalFormatFailureAndCancellation(t *testing.T) {
	t.Setenv("ARC_TEST_FORMAT_PROBE", "fail")
	p := New(os.Args[0])
	r, err := p.Probe(context.Background(), "file.mkv")
	if err != nil || r.Streams[0].HDRProbeStatus != "unavailable" {
		t.Fatalf("optional failure discarded basic metadata: %v %+v", err, r)
	}
	t.Setenv("ARC_TEST_FORMAT_PROBE", "wait")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := p.Probe(ctx, "file.mkv"); err == nil {
		t.Fatal("cancelled probe succeeded")
	}
}

func TestFormatOutputLimit(t *testing.T) {
	out := &limitedOutput{remaining: 4}
	if _, err := out.Write([]byte("12345")); err == nil || out.Len() != 0 {
		t.Fatal("output limit not enforced")
	}
}

// io.Copy must not bypass the cap through a promoted bytes.Buffer.ReadFrom method.
func TestFormatOutputLimitThroughCopy(t *testing.T) {
	out := &limitedOutput{remaining: 4}
	if _, err := io.Copy(out, io.LimitReader(strings.NewReader("12345"), 5)); err == nil || out.Len() != 0 {
		t.Fatal("copy bypassed output cap")
	}
}

func TestExistingFormatEnrichmentRefreshesAudioProfileAndSamplesVideo(t *testing.T) {
	t.Setenv("ARC_TEST_FORMAT_PROBE", "sample")
	streams := []Stream{{Index: 0, Kind: "video", Codec: "hevc"}, {Index: 2, Kind: "audio", Codec: "truehd"}, {Index: 4, Kind: "video", Codec: "av1"}}
	if err := New(os.Args[0]).EnrichFormats(context.Background(), "file.mkv", streams); err != nil {
		t.Fatal(err)
	}
	if streams[1].Profile != "Dolby TrueHD + Dolby Atmos" || streams[1].ChannelLayout != "7.1" || !streams[2].DolbyVision || !streams[2].HDR10Plus {
		t.Fatalf("lost existing-file evidence: %+v", streams)
	}
	if streams[0].AspectRatio != 1.78 || streams[1].AspectRatio != 0 || streams[2].AspectRatio != 2.4 {
		t.Fatalf("aspect ratios: %v %v %v", streams[0].AspectRatio, streams[1].AspectRatio, streams[2].AspectRatio)
	}
}

func TestAspectRatio(t *testing.T) {
	for _, c := range []struct {
		kind            string
		w, h            int
		sample, display string
		want            float64
	}{
		{"video", 1920, 1080, "1:1", "16:9", 1.78},
		{"video", 1920, 800, "", "", 2.4},          // no ratios: the frame size
		{"video", 720, 480, "32:27", "", 1.78},     // anamorphic DVD from its pixel shape
		{"video", 720, 576, "0:1", "N/A", 1.25},    // unknown ratios fall back to the frame
		{"video", 3840, 1600, "1:1", "12:5", 2.4},  // ffprobe's display ratio wins
		{"video", 1440, 1080, "4:3", "16:9", 1.78}, // HDV
		{"audio", 0, 0, "", "", 0},
		{"video", 0, 0, "1:1", "16:9", 0},
	} {
		if got := aspectRatio(c.kind, c.w, c.h, c.sample, c.display); got != c.want {
			t.Errorf("%s %dx%d sar=%q dar=%q: %v, want %v", c.kind, c.w, c.h, c.sample, c.display, got, c.want)
		}
	}
}

// A real anamorphic file: 720x480 stored pixels shown as 16:9.
func TestProbeReadsAnamorphicAspectRatio(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	path := filepath.Join(t.TempDir(), "dvd.mkv")
	out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc=size=720x480:rate=25:duration=1",
		"-vf", "setsar=32/27", "-c:v", "libx264", "-preset", "ultrafast", path).CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg: %v %s", err, out)
	}
	res, err := New(ffprobe).Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Streams) != 1 || res.Streams[0].Width != 720 || res.Streams[0].AspectRatio != 1.78 {
		t.Fatalf("streams: %+v", res.Streams)
	}
}

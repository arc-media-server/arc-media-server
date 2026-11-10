package transcode

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentConversionLimit(t *testing.T) {
	// The test executable is a harmless process stand-in: it exits on FFmpeg's unknown flags.
	// Admission happens at Start, before any media output is required.
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const requests = 12
	gate := make(chan struct{})
	var arrived atomic.Int32
	m := New(filepath.Join(t.TempDir(), "sessions"), func() string {
		if arrived.Add(1) == requests {
			close(gate)
		}
		<-gate
		return bin
	}, func() string { return "" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.Configure(m.dir, 1)
	var wg sync.WaitGroup
	errs := make(chan error, requests)
	started := make(chan *Session, requests)
	for range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := m.Start(Options{Path: "unused", AudioStream: -1, BurnSubtitle: -1})
			if err != nil {
				errs <- err
			} else {
				started <- s
			}
		}()
	}
	wg.Wait()
	close(errs)
	close(started)
	var sessions []*Session
	for s := range started {
		sessions = append(sessions, s)
	}
	t.Cleanup(func() {
		for _, s := range sessions {
			m.Stop(s.ID)
			<-s.done
		}
	})
	for err := range errs {
		if !errors.Is(err, ErrBusy) {
			t.Errorf("unexpected startup error: %v", err)
		}
	}
	if len(sessions) != 1 {
		t.Fatalf("accepted %d conversions with limit 1", len(sessions))
	}
	// Repackaging doesn't spend a video-encoding slot.
	remux, err := m.Start(Options{Path: "unused", CopyVideo: true, AudioStream: -1, BurnSubtitle: -1})
	if err != nil {
		t.Fatal(err)
	}
	sessions = append(sessions, remux)
	m.Stop(sessions[0].ID)
	next, err := m.Start(Options{Path: "unused", AudioStream: -1, BurnSubtitle: -1})
	if err != nil {
		t.Fatalf("slot wasn't released: %v", err)
	}
	sessions = append(sessions, next)
}

func TestFailedConversionStartReleasesCapacity(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "missing-ffmpeg")
	m := New(filepath.Join(t.TempDir(), "sessions"), func() string { return bin }, func() string { return "" },
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.Configure(m.dir, 1)
	for range 2 {
		_, err := m.Start(Options{Path: "unused", AudioStream: -1, BurnSubtitle: -1})
		if err == nil || errors.Is(err, ErrBusy) {
			t.Fatalf("expected process-start error, got %v", err)
		}
		if m.Count() != 0 {
			t.Fatal("failed conversion retained a slot")
		}
	}
}

func TestBuildArgs(t *testing.T) {
	copyArgs := strings.Join(buildArgs(Options{Path: "in.mkv", VideoStream: 0, AudioStream: 2, CopyVideo: true, VideoCodec: "hevc", BurnSubtitle: -1}, 20, "out"), " ")
	for _, want := range []string{"-ss 20.000 -noaccurate_seek -copyts -i in.mkv", "-map 0:0 -map 0:2", "-c:v copy -avoid_negative_ts disabled -tag:v hvc1 -strict unofficial", "-c:a aac -ac 2", "-hls_segment_type fmp4"} {
		if !strings.Contains(copyArgs, want) {
			t.Errorf("copy args missing %q:\n%s", want, copyArgs)
		}
	}
	surround := strings.Join(buildArgs(Options{Path: "in.mkv", VideoStream: 0, AudioStream: 1, CopyVideo: true, VideoCodec: "hevc", AudioCodec: "eac3", AudioChannels: 6, BurnSubtitle: -1}, 0, "out"), " ")
	if !strings.Contains(surround, "-c:a eac3 -ac 6 -b:a 640k") {
		t.Errorf("surround args: %s", surround)
	}
	enc := strings.Join(buildArgs(Options{Path: "in.mkv", AudioStream: -1, MaxHeight: 720, SourceHeight: 2160, VideoKbps: 4000, BurnSubtitle: -1}, 0, "out"), " ")
	for _, want := range []string{"-c:v libx264", "-b:v 4000k", FitScaleFilter(720)} {
		if !strings.Contains(enc, want) {
			t.Errorf("encode args missing %q:\n%s", want, enc)
		}
	}
	burn := strings.Join(buildArgs(Options{Path: "in.mkv", VideoStream: 0, AudioStream: 1, BurnSubtitle: 3, MaxHeight: 720, SourceHeight: 1080}, 0, "out"), " ")
	if !strings.Contains(burn, "-filter_complex [0:3][0:0]scale2ref=w=iw:h=ih[subscaled][videoref];[videoref][subscaled]overlay=eof_action=pass,"+FitScaleFilter(720)+"[v] -map [v] -map 0:1") || strings.Contains(burn, "-vf") {
		t.Errorf("burn-in args:\n%s", burn)
	}
	for _, unwanted := range []string{"-ss", "-c:a", "-copyts"} {
		if strings.Contains(enc, unwanted) {
			t.Errorf("unexpected %q:\n%s", unwanted, enc)
		}
	}
}

// TestSession runs FFmpeg for real when it's installed: remuxing from part-way through starts on
// a keyframe, and Ready measures where.
func TestSession(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, _ := exec.LookPath("ffprobe")
	dir := t.TempDir()
	m := New(filepath.Join(dir, "sessions"), func() string { return ffmpeg }, func() string { return ffprobe },
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, tc := range []struct {
		name, codec string
		encode      []string
		wantOffset  float64 // the keyframe before 9s
	}{
		// A keyframe every 2 seconds.
		{"h264", "h264", []string{"-c:v", "libx264", "-g", "50", "-keyint_min", "50"}, 8},
		// x265's open GOPs: frames after a keyframe can be shown before it.
		{"hevc", "hevc", []string{"-c:v", "libx265", "-x265-params", "log-level=error:keyint=100:min-keyint=100"}, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := filepath.Join(dir, tc.name+".mkv")
			args := append([]string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=duration=20:size=320x180:rate=25",
				"-f", "lavfi", "-i", "sine=duration=20"}, tc.encode...)
			if out, err := exec.Command(ffmpeg, append(args, "-c:a", "ac3", src)...).CombinedOutput(); err != nil {
				t.Fatalf("making source: %v %s", err, out)
			}
			s, err := m.Start(Options{Path: src, StartMS: 9000, VideoStream: 0, AudioStream: 1, CopyVideo: true, VideoCodec: tc.codec, BurnSubtitle: -1})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := m.Ready(ctx, s); err != nil {
				t.Fatal(err)
			}
			// Within a frame or two (B-frames start a little before the keyframe).
			if got := float64(s.OffsetMS) / 1000; got < tc.wantOffset-0.2 || got > tc.wantOffset+0.05 {
				t.Errorf("offset = %.3fs, want about %vs", got, tc.wantOffset)
			}
			<-s.done
			if _, ok := s.File("init.mp4"); !ok {
				t.Error("no init segment")
			}
			if _, ok := s.File("../secret"); ok {
				t.Error("served a path outside the session")
			}
			// The output has video, not just audio.
			out, _ := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0",
				filepath.Join(s.dir, "index.m3u8")).Output()
			if !strings.Contains(string(out), tc.codec) {
				t.Errorf("output streams: %s", out)
			}
			m.Stop(s.ID)
			deadline := time.Now().Add(3 * time.Second)
			for {
				if _, err := os.Stat(s.dir); os.IsNotExist(err) {
					break
				}
				if time.Now().After(deadline) {
					t.Error("session folder left behind")
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		})
	}
}

func TestHardwareAndToneMapArgs(t *testing.T) {
	nv := strings.Join(buildArgs(Options{Path: "in.mkv", AudioStream: -1, BurnSubtitle: -1, HWEncoder: "h264_nvenc", ToneMap: true,
		SourceHeight: 2160, MaxHeight: 1080}, 0, "out"), " ")
	for _, want := range []string{"-hwaccel auto -i in.mkv", "-vf " + ToneMapFilter + "," + FitScaleFilter(1080), "-c:v h264_nvenc -b:v 10000k", "-forced-idr 1"} {
		if !strings.Contains(nv, want) {
			t.Errorf("nvenc args missing %q:\n%s", want, nv)
		}
	}
	burn := strings.Join(buildArgs(Options{Path: "in.mkv", AudioStream: -1, BurnSubtitle: 4, ToneMap: true}, 0, "out"), " ")
	if !strings.Contains(burn, "-filter_complex [0:0]"+ToneMapFilter+"[pre];[0:4][pre]scale2ref=w=iw:h=ih[subscaled][videoref];[videoref][subscaled]overlay=eof_action=pass[v]") {
		t.Errorf("tone mapped burn-in:\n%s", burn)
	}
	va := strings.Join(buildArgs(Options{Path: "in.mkv", AudioStream: -1, BurnSubtitle: -1, HWEncoder: "h264_vaapi"}, 0, "out"), " ")
	if !strings.Contains(va, "-vaapi_device /dev/dri/renderD128 -hwaccel auto") || !strings.Contains(va, "-vf format=nv12,hwupload") {
		t.Errorf("vaapi:\n%s", va)
	}
	// Copying ignores all of it.
	cp := strings.Join(buildArgs(Options{Path: "in.mkv", AudioStream: -1, BurnSubtitle: -1, CopyVideo: true, HWEncoder: "h264_nvenc", ToneMap: true}, 0, "out"), " ")
	if strings.Contains(cp, "hwaccel") || strings.Contains(cp, "zscale") {
		t.Errorf("copy:\n%s", cp)
	}
}

// TestConvertHDR converts HDR10 HEVC to SDR H.264 for real, with this computer's hardware encoder
// if it has one.
func TestConvertHDR(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, _ := exec.LookPath("ffprobe")
	dir := t.TempDir()
	src := filepath.Join(dir, "hdr.mkv")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=duration=4:size=640x360:rate=24",
		"-c:v", "libx265", "-pix_fmt", "yuv420p10le", "-x265-params", "log-level=error",
		"-bsf:v", "hevc_metadata=colour_primaries=9:transfer_characteristics=16:matrix_coefficients=9", src).CombinedOutput(); err != nil {
		t.Fatalf("making source: %v %s", err, out)
	}
	m := New(filepath.Join(dir, "sessions"), func() string { return ffmpeg }, func() string { return ffprobe },
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	caps := m.Detect(context.Background())
	if !caps.ToneMapping {
		t.Skip("this FFmpeg has no zscale")
	}
	t.Logf("hardware encoder: %q", caps.HWEncoder)
	s, err := m.Start(Options{Path: src, VideoStream: 0, AudioStream: -1, BurnSubtitle: -1, HWEncoder: caps.HWEncoder, ToneMap: true, SourceHeight: 360})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := m.Ready(ctx, s); err != nil {
		t.Fatal(err)
	}
	<-s.done
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	out, _ := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_name,pix_fmt,color_transfer", "-of", "csv=p=0",
		filepath.Join(s.dir, "index.m3u8")).Output()
	if got := strings.TrimSpace(string(out)); !strings.HasPrefix(got, "h264,yuv420p") || strings.Contains(got, "smpte2084") {
		t.Errorf("output: %q", got)
	}
	m.Stop(s.ID)
}

// Copied Dolby Vision that comes out without its configuration box is logged, once per run.
func TestDolbyVisionDroppedIsLoggedOnce(t *testing.T) {
	var logs strings.Builder
	m := New(t.TempDir(), func() string { return "" }, func() string { return "" }, slog.New(slog.NewTextHandler(&logs, nil)))
	session := func(init string, dv bool) *Session {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "init.mp4"), []byte(init), 0o644); err != nil {
			t.Fatal(err)
		}
		return &Session{ID: "s", dir: dir, Options: Options{CopyVideo: true, DolbyVision: dv}}
	}
	m.checkDolbyVision(session("ftyp hvc1 hvcC dvvC", true))
	m.checkDolbyVision(session("ftyp hvc1 hvcC", false))
	if logs.Len() != 0 {
		t.Fatalf("kept or no Dolby Vision is no warning: %s", logs.String())
	}
	m.checkDolbyVision(session("ftyp hvc1 hvcC", true))
	m.checkDolbyVision(session("ftyp hvc1 hvcC", true))
	if n := strings.Count(logs.String(), "didn't keep Dolby Vision"); n != 1 {
		t.Fatalf("want one warning, got %d: %s", n, logs.String())
	}
}

func TestFastToneMapUsesTonemapx(t *testing.T) {
	args := strings.Join(buildArgs(Options{Path: "in.mkv", VideoStream: 0, AudioStream: 1, ToneMap: true, FastToneMap: true, BurnSubtitle: -1}, 0, "out"), " ")
	if !strings.Contains(args, "-vf "+FastToneMapFilter) || strings.Contains(args, "zscale") {
		t.Fatalf("fast tone mapping should use tonemapx alone: %s", args)
	}
	slow := strings.Join(buildArgs(Options{Path: "in.mkv", VideoStream: 0, AudioStream: 1, ToneMap: true, BurnSubtitle: -1}, 0, "out"), " ")
	if !strings.Contains(slow, ToneMapFilter) {
		t.Fatalf("without tonemapx the zscale chain stays: %s", slow)
	}
}

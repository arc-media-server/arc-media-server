package transcode

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestDetectRenderNodeDriverFallbackAndLowPower(t *testing.T) {
	m := New(t.TempDir(), func() string { return "fixture-ffmpeg" }, func() string { return "fixture-probe" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.gpuNames = nil
	m.renderNodes = func() []string { return []string{"/dev/dri/renderD129"} }
	m.probeHardware = func(ctx context.Context, bin string, args []string, driver string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "-filters") {
			return []byte(" zscale "), nil
		}
		if strings.Contains(joined, "h264_vaapi") && strings.Contains(joined, "renderD129") && driver == "i965" && strings.Contains(joined, "-low_power 1") {
			return nil, nil
		}
		return []byte(strings.Repeat("driver unavailable ", 100)), errors.New("fixture failure")
	}
	caps := m.Detect(context.Background())
	if caps.HWEncoder != "h264_vaapi" || caps.HWDevice != "/dev/dri/renderD129" || caps.HWDriver != "i965" || !caps.HWLowPower || !caps.Checked || caps.Checking || !caps.ToneMapping {
		t.Fatalf("capabilities: %+v", caps)
	}
	for _, a := range caps.Attempts {
		if len(a.Error) > 512 {
			t.Fatal("unbounded probe error")
		}
	}
	args := strings.Join(buildArgs(Options{Path: "fixture.mkv", AudioStream: -1, BurnSubtitle: -1, HWEncoder: caps.HWEncoder, HWDevice: caps.HWDevice, HWDriver: caps.HWDriver, HWLowPower: caps.HWLowPower}, 0, "out"), " ")
	if !strings.Contains(args, "-vaapi_device /dev/dri/renderD129") || !strings.Contains(args, "-low_power 1") || strings.Contains(args, "renderD128") {
		t.Fatalf("tested configuration not used: %s", args)
	}
	env := withVADriver([]string{"LIBVA_DRIVER_NAME=old", "OTHER=kept"}, caps.HWDriver)
	if strings.Join(env, " ") != "OTHER=kept LIBVA_DRIVER_NAME=i965" {
		t.Fatalf("driver environment: %v", env)
	}
	// Callers cannot mutate the manager's retained diagnostic history.
	caps.Attempts[0].Error = "changed"
	if m.Caps().Attempts[0].Error == "changed" {
		t.Fatal("capabilities leaked mutable slice")
	}
}

func TestHardwareRetestCoalescesAndRetainsExistingCapsDuringCheck(t *testing.T) {
	m := New(t.TempDir(), func() string { return "fixture" }, func() string { return "fixture" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.gpuNames = nil
	m.caps = Capabilities{HWEncoder: "old-encoder", Checked: true}
	release := make(chan struct{})
	m.probeHardware = func(ctx context.Context, _ string, args []string, _ string) ([]byte, error) {
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return nil, nil
	}
	if !m.RequestDetect(context.Background()) || m.RequestDetect(context.Background()) {
		t.Fatal("retest did not coalesce")
	}
	if caps := m.Caps(); !caps.Checking || caps.HWEncoder != "old-encoder" {
		t.Fatal("inflight check discarded working capabilities")
	}
	close(release)
	m.detectMu.Lock()
	m.detectMu.Unlock()
	if caps := m.Caps(); caps.Checking || caps.HWEncoder != "h264_nvenc" {
		t.Fatalf("retest did not finish: %+v", caps)
	}
}

func TestHardwareProbeOutputBound(t *testing.T) {
	out := &probeOutput{}
	text := []byte(strings.Repeat("x", 512*1024))
	n, err := out.Write(text)
	if err != nil || n != len(text) || out.Len() != 256*1024 {
		t.Fatalf("output bound: %d %d %v", n, out.Len(), err)
	}
}

func TestDetectListsEachDeviceOnceAndUsesTheChosenOne(t *testing.T) {
	m := New(t.TempDir(), func() string { return "fixture-ffmpeg" }, func() string { return "fixture-probe" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.gpuNames = nil
	// An Intel iGPU (Quick Sync and VA-API both work) and an AMD card (VA-API only).
	m.renderNodes = func() []string { return []string{"/dev/dri/renderD128", "/dev/dri/renderD129"} }
	m.gpuVendor = func(node string) string {
		return map[string]string{"/dev/dri/renderD128": "Intel", "/dev/dri/renderD129": "AMD"}[node]
	}
	m.probeHardware = func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "-filters"):
			return nil, nil
		case strings.Contains(joined, "h264_qsv") && strings.Contains(joined, "renderD128"),
			strings.Contains(joined, "h264_vaapi"):
			return nil, nil
		}
		return []byte("unavailable"), errors.New("fixture failure")
	}
	m.UseDevice("h264_vaapi:/dev/dri/renderD129")
	caps := m.Detect(context.Background())
	var labels []string
	for _, d := range caps.Devices {
		labels = append(labels, d.ID+"="+d.Label)
	}
	if got := strings.Join(labels, ", "); got != "h264_qsv:/dev/dri/renderD128=Intel Quick Sync · renderD128, h264_vaapi:/dev/dri/renderD129=AMD (VA-API) · renderD129" {
		t.Fatalf("devices: %s", got)
	}
	if caps.HWEncoder != "h264_vaapi" || caps.HWDevice != "/dev/dri/renderD129" {
		t.Fatalf("chosen device not used: %+v", caps)
	}
	m.UseDevice("") // Auto: the first that works.
	if caps := m.Caps(); caps.HWEncoder != "h264_qsv" || caps.HWDevice != "/dev/dri/renderD128" {
		t.Fatalf("auto: %+v", caps)
	}
	m.UseDevice("h264_nvenc:") // Not here (removed card): Auto.
	if caps := m.Caps(); caps.HWEncoder != "h264_qsv" {
		t.Fatalf("missing device did not fall back: %+v", caps)
	}
}

func TestAutomaticIsTheSystemsFirstDeviceNotAnAPIRanking(t *testing.T) {
	m := New(t.TempDir(), func() string { return "fixture-ffmpeg" }, func() string { return "fixture-probe" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.gpuNames = nil
	// The system lists an AMD card first and an NVIDIA card second. NVENC would rank first by API,
	// but Automatic is the first device the system lists.
	m.renderNodes = func() []string { return []string{"/dev/dri/renderD128", "/dev/dri/renderD129"} }
	m.gpuVendor = func(node string) string {
		return map[string]string{"/dev/dri/renderD128": "AMD", "/dev/dri/renderD129": "NVIDIA"}[node]
	}
	m.probeHardware = func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "-filters"), strings.Contains(joined, "h264_nvenc"),
			strings.Contains(joined, "h264_vaapi") && strings.Contains(joined, "renderD128"):
			return nil, nil
		}
		return []byte("unavailable"), errors.New("fixture failure")
	}
	caps := m.Detect(context.Background())
	if len(caps.Devices) != 2 || caps.Devices[0].Encoder != "h264_vaapi" || caps.Devices[1].Encoder != "h264_nvenc" {
		t.Fatalf("devices not in system order: %+v", caps.Devices)
	}
	if caps.HWEncoder != "h264_vaapi" || caps.HWDevice != "/dev/dri/renderD128" {
		t.Fatalf("automatic: %+v", caps)
	}
}

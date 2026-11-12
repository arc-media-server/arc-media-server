package transcode

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var hwEncoders = []struct{ name, label string }{
	{"h264_nvenc", "NVIDIA NVENC"}, {"h264_qsv", "Intel Quick Sync"}, {"h264_videotoolbox", "Apple VideoToolbox"}, {"h264_amf", "AMD AMF"}, {"h264_vaapi", "VA-API"},
}

type HardwareAttempt struct {
	Encoder  string `json:"encoder"`
	Device   string `json:"device,omitempty"`
	Driver   string `json:"driver,omitempty"`
	LowPower bool   `json:"low_power,omitempty"`
	Error    string `json:"error,omitempty"`
}

// HardwareDevice is one graphics device that passed the test encode, offered in Settings.
type HardwareDevice struct {
	// ID is what Settings stores: the encoder and device ("h264_qsv:/dev/dri/renderD128").
	ID       string `json:"id"`
	Label    string `json:"label"`
	Encoder  string `json:"encoder"`
	Device   string `json:"device,omitempty"`
	Driver   string `json:"driver,omitempty"`
	LowPower bool   `json:"low_power,omitempty"`
}

type Capabilities struct {
	// The device conversions use: the one chosen in Settings, or the first in the system's order.
	HWEncoder  string            `json:"hw_encoder"`
	HWLabel    string            `json:"hw_label"`
	HWDevice   string            `json:"hw_device,omitempty"`
	HWDriver   string            `json:"hw_driver,omitempty"`
	HWLowPower bool              `json:"hw_low_power,omitempty"`
	Attempts   []HardwareAttempt `json:"attempts,omitempty"`
	// Every device that works, in the system's order; Automatic uses the first.
	Devices     []HardwareDevice `json:"devices,omitempty"`
	ToneMapping bool             `json:"tone_mapping"`
	// Jellyfin's FFmpeg's tonemapx: HDR to SDR in one fast (SIMD) filter instead of zscale's chain.
	FastToneMap bool `json:"fast_tone_map,omitempty"`
	Checked     bool `json:"checked"`
	Checking    bool `json:"checking"`
}

// A single bounded detection job, using only generated blank frames. Existing
// playback capabilities remain available until its result replaces them.
func (m *Manager) Detect(ctx context.Context) Capabilities {
	if !m.detectMu.TryLock() {
		return m.Caps()
	}
	defer m.detectMu.Unlock()
	return m.detect(ctx)
}
func (m *Manager) RequestDetect(ctx context.Context) bool {
	if !m.detectMu.TryLock() {
		return false
	}
	m.mu.Lock()
	m.caps.Checking = true
	m.mu.Unlock()
	go func() { defer m.detectMu.Unlock(); m.detect(ctx) }()
	return true
}
func (m *Manager) detect(ctx context.Context) Capabilities {
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	m.mu.Lock()
	m.caps.Checking = true
	m.mu.Unlock()
	started := time.Now()
	m.log.Info("hardware detection started")
	caps := Capabilities{Checked: true}
	bin := m.ffmpeg()
	nodes := []string{}
	if m.renderNodes != nil {
		nodes = m.renderNodes()
	} else {
		nodes, _ = filepath.Glob("/dev/dri/renderD*")
	}
	if len(nodes) > 4 {
		nodes = nodes[:4]
	}
	run := m.probeHardware
	if run == nil {
		run = runHardwareProbe
	}
	// Each render node is offered once, through the first API that works on it (Quick Sync
	// before VA-API on Intel), so Settings lists graphics devices rather than APIs.
	claimed := map[string]bool{}
	attempt := func(encoder, device, driver string, low bool) bool {
		work, stop := context.WithTimeout(ctx, 8*time.Second)
		defer stop()
		out, err := run(work, bin, hardwareProbeArgs(encoder, device, low), driver)
		a := HardwareAttempt{Encoder: encoder, Device: device, Driver: driver, LowPower: low}
		if err != nil || strings.Contains(string(out), "Nothing was written") {
			detail := strings.Join(strings.Fields(string(out)), " ")
			if detail == "" {
				detail = fmt.Sprint(err)
			}
			if len(detail) > 512 {
				detail = detail[:512]
			}
			a.Error = detail
		}
		caps.Attempts = append(caps.Attempts, a)
		if a.Error == "" {
			caps.Devices = append(caps.Devices, HardwareDevice{ID: encoder + ":" + device, Encoder: encoder, Device: device, Driver: driver, LowPower: low})
			claimed[device] = true
			return true
		}
		m.log.Debug("hardware encoder test unavailable", "encoder", encoder, "device", device, "driver", driver, "reason", a.Error)
		return false
	}
	if bin == "" {
		caps.Attempts = append(caps.Attempts, HardwareAttempt{Error: "FFmpeg was not found"})
	} else {
		for _, e := range hwEncoders {
			if ctx.Err() != nil {
				break
			}
			switch {
			case e.name == "h264_vaapi":
				for _, node := range nodes {
					if claimed[node] {
						continue
					}
				drivers:
					for _, driver := range []string{"", "iHD", "i965"} {
						for _, low := range []bool{false, true} {
							if ctx.Err() != nil || attempt(e.name, node, driver, low) {
								break drivers
							}
						}
					}
				}
				if len(nodes) == 0 {
					caps.Attempts = append(caps.Attempts, HardwareAttempt{Encoder: e.name, Error: "No accessible render device was found under /dev/dri"})
				}
			case e.name == "h264_qsv" && len(nodes) > 0:
				for _, node := range nodes {
					if ctx.Err() != nil {
						break
					}
					if !claimed[node] {
						attempt(e.name, node, "", false)
					}
				}
			default:
				attempt(e.name, "", "", false)
			}
		}
		systemOrder(caps.Devices, nodes, m.gpuVendor)
		labelDevices(caps.Devices, m.gpuVendor)
		out, _ := run(ctx, bin, []string{"-hide_banner", "-filters"}, "")
		caps.FastToneMap = strings.Contains(string(out), " tonemapx ")
		caps.ToneMapping = caps.FastToneMap || strings.Contains(string(out), " zscale ")
	}
	m.mu.Lock()
	if parent.Err() != nil {
		m.caps.Checking = false
		caps = m.caps
	} else {
		caps.choose(m.hwChoice)
		m.caps = caps
	}
	m.mu.Unlock()
	m.log.Info("hardware detection finished", "encoder", caps.HWLabel, "device", caps.HWDevice, "devices", len(caps.Devices), "choice", choiceName(m.hardwareChoice()), "attempts", len(caps.Attempts), "took", time.Since(started).Round(time.Millisecond))
	caps.Attempts = append([]HardwareAttempt(nil), caps.Attempts...)
	return caps
}
func (m *Manager) Caps() Capabilities {
	m.mu.Lock()
	defer m.mu.Unlock()
	caps := m.caps
	caps.Attempts = append([]HardwareAttempt(nil), caps.Attempts...)
	caps.Devices = append([]HardwareDevice(nil), caps.Devices...)
	return caps
}

// UseDevice picks the graphics device conversions use, by HardwareDevice.ID ("" for Auto: the
// first that works). A device that isn't there (removed, or failing its test) falls back to Auto.
// Conversions already running keep the device they started with.
func (m *Manager) UseDevice(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hwChoice == id {
		return
	}
	m.hwChoice = id
	if m.caps.Checked {
		m.caps.choose(id)
		m.log.Info("hardware device chosen", "choice", choiceName(id), "using", m.caps.HWLabel, "device", m.caps.HWDevice)
	}
}

func (m *Manager) hardwareChoice() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hwChoice
}

func choiceName(id string) string {
	if id == "" {
		return "auto"
	}
	return id
}

// choose sets the active device from the working ones: the chosen ID, else the first.
func (caps *Capabilities) choose(id string) {
	caps.HWEncoder, caps.HWLabel, caps.HWDevice, caps.HWDriver, caps.HWLowPower = "", "", "", "", false
	if len(caps.Devices) == 0 {
		return
	}
	d := caps.Devices[0]
	for _, candidate := range caps.Devices {
		if candidate.ID == id {
			d = candidate
			break
		}
	}
	caps.HWEncoder, caps.HWLabel, caps.HWDevice, caps.HWDriver, caps.HWLowPower = d.Encoder, d.Label, d.Device, d.Driver, d.LowPower
}

// systemOrder puts devices in the order the system lists them (render nodes, renderD128 first), so
// Automatic is simply the first graphics device, not a ranking of APIs. NVENC has no node of its
// own, so it takes the place of the first NVIDIA node; devices with no node at all (Windows, macOS)
// keep the order they were found in, after any that have one.
func systemOrder(devices []HardwareDevice, nodes []string, vendor func(node string) string) {
	place := func(d HardwareDevice) int {
		for i, node := range nodes {
			if node == d.Device || (d.Encoder == "h264_nvenc" && vendor != nil && vendor(node) == "NVIDIA") {
				return i
			}
		}
		return len(nodes)
	}
	sort.SliceStable(devices, func(i, j int) bool { return place(devices[i]) < place(devices[j]) })
}

// labelDevices names devices for Settings: the API, with the maker for VA-API ("AMD (VA-API)"),
// and the render node when more than one device was found.
func labelDevices(devices []HardwareDevice, vendor func(node string) string) {
	for i := range devices {
		d := &devices[i]
		d.Label = EncoderLabel(d.Encoder)
		if d.Encoder == "h264_vaapi" && vendor != nil {
			if maker := vendor(d.Device); maker != "" {
				d.Label = maker + " (VA-API)"
			}
		}
		if len(devices) > 1 && d.Device != "" {
			d.Label += " · " + filepath.Base(d.Device)
		}
	}
}

// sysfsGPUVendor names the maker of a render node from its PCI vendor ID, "" when unknown.
func sysfsGPUVendor(node string) string {
	raw, err := os.ReadFile(filepath.Join("/sys/class/drm", filepath.Base(node), "device", "vendor"))
	if err != nil {
		return ""
	}
	switch strings.TrimSpace(string(raw)) {
	case "0x8086":
		return "Intel"
	case "0x1002":
		return "AMD"
	case "0x10de":
		return "NVIDIA"
	}
	return ""
}
func hardwareProbeArgs(encoder, device string, low bool) []string {
	args := []string{"-hide_banner", "-nostdin", "-v", "error"}
	vf := "format=yuv420p"
	if encoder == "h264_vaapi" {
		args = append(args, "-vaapi_device", device)
		vf = "format=nv12,hwupload"
	}
	if encoder == "h264_qsv" && device != "" {
		args = append(args, "-qsv_device", device)
	}
	args = append(args, "-f", "lavfi", "-i", "color=black:s=256x144:d=0.2", "-vf", vf, "-c:v", encoder, "-profile:v", "high")
	if low {
		args = append(args, "-low_power", "1")
	}
	return append(args, "-f", "null", "-")
}
func withVADriver(env []string, driver string) []string {
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, "LIBVA_DRIVER_NAME=") {
			out = append(out, entry)
		}
	}
	return append(out, "LIBVA_DRIVER_NAME="+driver)
}

// FFmpeg output is capped while the child runs, not after collecting an unbounded buffer.
type probeOutput struct{ bytes.Buffer }

func (b *probeOutput) Write(p []byte) (int, error) {
	n := len(p)
	limit := max(0, 256*1024-b.Len())
	if len(p) > limit {
		p = p[:limit]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}
func runHardwareProbe(ctx context.Context, bin string, args []string, driver string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	if driver != "" {
		cmd.Env = withVADriver(os.Environ(), driver)
	}
	out := &probeOutput{}
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	return out.Bytes(), err
}
func EncoderLabel(encoder string) string {
	for _, e := range hwEncoders {
		if e.name == encoder {
			return e.label
		}
	}
	return ""
}

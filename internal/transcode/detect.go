package transcode

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

type Capabilities struct {
	HWEncoder   string            `json:"hw_encoder"`
	HWLabel     string            `json:"hw_label"`
	HWDevice    string            `json:"hw_device,omitempty"`
	HWDriver    string            `json:"hw_driver,omitempty"`
	HWLowPower  bool              `json:"hw_low_power,omitempty"`
	Attempts    []HardwareAttempt `json:"attempts,omitempty"`
	ToneMapping bool              `json:"tone_mapping"`
	Checked     bool              `json:"checked"`
	Checking    bool              `json:"checking"`
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
			caps.HWEncoder, caps.HWLabel, caps.HWDevice, caps.HWDriver, caps.HWLowPower = encoder, EncoderLabel(encoder), device, driver, low
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
			if e.name == "h264_vaapi" {
				for _, node := range nodes {
					for _, driver := range []string{"", "iHD", "i965"} {
						for _, low := range []bool{false, true} {
							if ctx.Err() != nil || attempt(e.name, node, driver, low) {
								break
							}
						}
						if caps.HWEncoder != "" || ctx.Err() != nil {
							break
						}
					}
					if caps.HWEncoder != "" || ctx.Err() != nil {
						break
					}
				}
				if len(nodes) == 0 {
					caps.Attempts = append(caps.Attempts, HardwareAttempt{Encoder: e.name, Error: "No accessible render device was found under /dev/dri"})
				}
			} else if e.name == "h264_qsv" && len(nodes) > 0 {
				for _, node := range nodes {
					if attempt(e.name, node, "", false) || ctx.Err() != nil {
						break
					}
				}
			} else {
				attempt(e.name, "", "", false)
			}
			if caps.HWEncoder != "" {
				break
			}
		}
		out, _ := run(ctx, bin, []string{"-hide_banner", "-filters"}, "")
		caps.ToneMapping = strings.Contains(string(out), " zscale ")
	}
	m.mu.Lock()
	if parent.Err() != nil {
		m.caps.Checking = false
		caps = m.caps
	} else {
		m.caps = caps
	}
	m.mu.Unlock()
	m.log.Info("hardware detection finished", "encoder", caps.HWLabel, "device", caps.HWDevice, "attempts", len(caps.Attempts), "took", time.Since(started).Round(time.Millisecond))
	caps.Attempts = append([]HardwareAttempt(nil), caps.Attempts...)
	return caps
}
func (m *Manager) Caps() Capabilities {
	m.mu.Lock()
	defer m.mu.Unlock()
	caps := m.caps
	caps.Attempts = append([]HardwareAttempt(nil), caps.Attempts...)
	return caps
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

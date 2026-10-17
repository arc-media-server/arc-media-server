package transcode

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Hardware H.264 encoders, in the order they're preferred, with names people know.
var hwEncoders = []struct{ name, label string }{
	{"h264_nvenc", "NVIDIA NVENC"},
	{"h264_qsv", "Intel Quick Sync"},
	{"h264_videotoolbox", "Apple VideoToolbox"},
	{"h264_amf", "AMD AMF"},
	{"h264_vaapi", "VA-API"},
}

// Capabilities are what this computer's FFmpeg can do beyond the basics.
type Capabilities struct {
	// The hardware encoder that worked in a test, "" for none.
	HWEncoder string `json:"hw_encoder"`
	HWLabel   string `json:"hw_label"`
	// HDR tone mapping needs FFmpeg's zscale filter.
	ToneMapping bool `json:"tone_mapping"`
	Checked     bool `json:"checked"`
}

// Detect finds what this computer's FFmpeg can do: it tries each hardware encoder on a moment of
// blank video (a listed encoder may still have no device or driver behind it).
func (m *Manager) Detect(ctx context.Context) Capabilities {
	bin := m.ffmpeg()
	caps := Capabilities{Checked: bin != ""}
	if bin != "" {
		for _, e := range hwEncoders {
			if hwWorks(ctx, bin, e.name) {
				caps.HWEncoder, caps.HWLabel = e.name, e.label
				break
			}
		}
		out, _ := exec.CommandContext(ctx, bin, "-hide_banner", "-filters").Output()
		caps.ToneMapping = strings.Contains(string(out), " zscale ")
	}
	m.mu.Lock()
	m.caps = caps
	m.mu.Unlock()
	if caps.HWEncoder != "" {
		m.log.Info("hardware video encoding available", "encoder", caps.HWLabel)
	}
	return caps
}

// Caps returns what Detect found.
func (m *Manager) Caps() Capabilities {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.caps
}

func hwWorks(ctx context.Context, bin, encoder string) bool {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	args := []string{"-hide_banner", "-v", "error"}
	vf := "format=yuv420p"
	if encoder == "h264_vaapi" {
		args = append(args, "-vaapi_device", "/dev/dri/renderD128")
		vf = "format=nv12,hwupload"
	}
	args = append(args, "-f", "lavfi", "-i", "color=black:s=256x144:d=0.2", "-vf", vf, "-c:v", encoder, "-f", "null", "-")
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	return err == nil && !strings.Contains(string(out), "Nothing was written")
}

// EncoderLabel names a hardware encoder ("NVIDIA NVENC"), or "" for the CPU.
func EncoderLabel(encoder string) string {
	for _, e := range hwEncoders {
		if e.name == encoder {
			return e.label
		}
	}
	return ""
}

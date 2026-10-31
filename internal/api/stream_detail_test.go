package api

import (
	"testing"

	"ams/internal/store"
	"ams/internal/transcode"
)

// The Dashboard's stream details say what the file has, what the player gets and what converts it.
func TestStreamDetail(t *testing.T) {
	dv := &store.Stream{Kind: "video", Codec: "hevc", Height: 2160, ColorTransfer: "smpte2084", DolbyVision: true}
	truehd := &store.Stream{Kind: "audio", Codec: "truehd", Channels: 8}

	direct := directDetail(dv, truehd, nil)
	if direct.Video != "HEVC 4K Dolby Vision" || direct.Audio != "Dolby TrueHD 7.1" || direct.VideoOutput != "" || direct.Encoder != "" {
		t.Fatalf("direct: %+v", direct)
	}

	// Repackaged: the video copied, TrueHD made Dolby Digital Plus 5.1, nothing encoding video.
	remux := conversionDetail(dv, truehd, []string{"Dolby TrueHD audio"}, transcode.Options{CopyVideo: true, AudioCodec: "eac3", AudioChannels: 6, BurnSubtitle: -1})
	if remux.VideoOutput != "" || remux.AudioOutput != "Dolby Digital Plus 5.1" || remux.Encoder != "" || remux.Hardware || remux.Reasons[0] != "Dolby TrueHD audio" {
		t.Fatalf("remux: %+v", remux)
	}

	// Converted on the CPU to 1080p with HDR mapped to SDR and subtitles drawn in.
	hdr := &store.Stream{Kind: "video", Codec: "hevc", Height: 2160, ColorTransfer: "smpte2084"}
	cpu := conversionDetail(hdr, truehd, nil, transcode.Options{MaxHeight: 1080, ToneMap: true, BurnSubtitle: 3})
	if cpu.Video != "HEVC 4K HDR10" || cpu.VideoOutput != "H.264 1080p" || cpu.Encoder != "CPU" || cpu.Hardware || !cpu.ToneMapped || !cpu.SubtitlesBurned || cpu.AudioOutput != "AAC Stereo" {
		t.Fatalf("cpu: %+v", cpu)
	}

	// A graphics card is named, and counts as hardware.
	gpu := conversionDetail(hdr, nil, nil, transcode.Options{HWEncoder: "h264_nvenc", BurnSubtitle: -1})
	if !gpu.Hardware || gpu.Encoder == "" || gpu.Encoder == "CPU" || gpu.VideoOutput != "H.264 4K" || gpu.AudioOutput != "" {
		t.Fatalf("gpu: %+v", gpu)
	}
}

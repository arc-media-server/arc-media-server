package transcode

import (
	"bytes"
	"encoding/binary"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Generated PGS: a white 100x20 rectangle at (900,1000) on a 1920x1080 subtitle canvas.
// It is deliberately smaller than the video; no real subtitle/media assets are needed.
func fixturePGS() []byte {
	pack := func(values ...any) []byte {
		var b bytes.Buffer
		for _, v := range values {
			if err := binary.Write(&b, binary.BigEndian, v); err != nil {
				panic(err)
			}
		}
		return b.Bytes()
	}
	segment := func(kind byte, data []byte, pts uint32) []byte {
		return append(pack([2]byte{'P', 'G'}, pts, pts, kind, uint16(len(data))), data...)
	}
	pcs := pack(uint16(1920), uint16(1080), byte(0x10), uint16(0), byte(0x80), byte(0), byte(0), byte(1), uint16(0), byte(0), byte(0), uint16(900), uint16(1000))
	window := pack(byte(1), byte(0), uint16(0), uint16(0), uint16(1920), uint16(1080))
	palette := []byte{0, 0, 0, 16, 128, 128, 0, 1, 235, 128, 128, 255}
	rle := bytes.Repeat([]byte{0, 0xc0, 100, 1, 0, 0}, 20)
	length := len(rle) + 4
	object := append(pack(uint16(0), byte(0), byte(0xc0), [3]byte{byte(length >> 16), byte(length >> 8), byte(length)}, uint16(100), uint16(20)), rle...)
	var out []byte
	for _, s := range []struct {
		kind byte
		data []byte
	}{{0x16, pcs}, {0x17, window}, {0x14, palette}, {0x15, object}, {0x80, nil}} {
		out = append(out, segment(s.kind, s.data, 0)...)
	}
	clear := pack(uint16(1920), uint16(1080), byte(0x10), uint16(1), byte(0), byte(0), byte(0), byte(0))
	out = append(out, segment(0x16, clear, 180000)...)
	return append(out, segment(0x80, nil, 180000)...)
}
func TestPictureSubtitleCanvasScalesToVideo(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, _ := exec.LookPath("ffprobe")
	dir := t.TempDir()
	pgs := filepath.Join(dir, "caption.sup")
	if err := os.WriteFile(pgs, fixturePGS(), 0600); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "source.mkv")
	out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=black:size=3840x2160:rate=2:duration=2", "-i", pgs, "-map", "0:v:0", "-map", "1:s:0", "-c:v", "libx264", "-preset", "ultrafast", "-c:s", "copy", src).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	m := New(filepath.Join(dir, "sessions"), func() string { return ffmpeg }, func() string { return ffprobe }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s, err := m.Start(Options{Path: src, VideoStream: 0, AudioStream: -1, BurnSubtitle: 1, SourceHeight: 2160, MaxHeight: 1080})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Stop(s.ID)
	select {
	case <-s.done:
	case <-time.After(30 * time.Second):
		t.Fatal("burn-in stalled")
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	frame, err := exec.Command(ffmpeg, "-v", "error", "-ss", "0.5", "-i", filepath.Join(s.dir, "index.m3u8"), "-frames:v", "1", "-pix_fmt", "gray", "-f", "rawvideo", "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != 1920*1080 {
		t.Fatalf("unexpected frame size %d", len(frame))
	}
	minY, maxY := 1080, -1
	for index, pixel := range frame {
		if pixel > 200 {
			y := index / 1920
			minY = min(minY, y)
			maxY = max(maxY, y)
		}
	}
	if minY < 995 || maxY < 1015 || maxY > 1025 {
		t.Fatalf("caption drawn at rows %d..%d; expected about 1000..1019", minY, maxY)
	}
}

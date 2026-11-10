package api

import (
	"ams/internal/store"
	"ams/internal/transcode"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDV5ReencodeRefusedBeforeEncoder(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, _ := exec.LookPath("ffprobe")
	version, _ := exec.Command(ffmpeg, "-version").Output()
	t.Log(strings.Split(string(version), "\n")[0])
	s, u := securityServer(t)
	ctx := t.Context()
	lib, err := s.store.SyncLibrary(ctx, "DV fixture", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "DV fixture"}, "dv5")
	if err != nil {
		t.Fatal(err)
	}
	f := store.MediaFile{LibraryID: lib, Path: filepath.Join(t.TempDir(), "unopened.mkv"), Container: "matroska", DurationMS: 120000, Streams: []store.Stream{{Index: 0, Kind: "video", Codec: "hevc", Height: 2160, Width: 3840, BitDepth: 10, DolbyVision: true, DolbyVisionProfile: 5}}}
	f.ID, err = s.store.SaveFile(ctx, &f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetFileItems(ctx, f.ID, []string{id}, 0); err != nil {
		t.Fatal(err)
	}
	s.transcoder = transcode.New(filepath.Join(t.TempDir(), "sessions"), func() string { return ffmpeg }, func() string { return ffprobe }, s.log)
	body := `{"caps":{"containers":["mkv","mp4"],"video":["hevc","h264"],"max_bit_depth":10,"max_height":720,"hdr_formats":["dolby_vision"],"dolby_vision_profiles":[5]}}`
	for _, preview := range []bool{true, false} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.SetPathValue("id", id)
		r = r.WithContext(context.WithValue(r.Context(), userKey, u))
		w := httptest.NewRecorder()
		s.servePlayback(w, r, preview)
		if preview {
			var p playbackPlan
			json.Unmarshal(w.Body.Bytes(), &p)
			if w.Code != 200 || p.CanStart == nil || *p.CanStart {
				t.Fatalf("DV5 encode promised: %d %s", w.Code, w.Body.String())
			}
		} else if w.Code != 422 {
			t.Fatalf("DV5 encoder admitted: %d %s", w.Code, w.Body.String())
		}
		if s.transcoder.Count() != 0 {
			t.Fatal("unsafe conversion allocated")
		}
	}
	// The same DV-capable device can still play the original without re-encoding.
	r := httptest.NewRequest("POST", "/", strings.NewReader(strings.Replace(body, `"max_height":720`, `"max_height":2160`, 1)))
	r.SetPathValue("id", id)
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	w := httptest.NewRecorder()
	s.servePlayback(w, r, true)
	var p playbackPlan
	json.Unmarshal(w.Body.Bytes(), &p)
	if p.CanStart == nil || !*p.CanStart || p.Method != "direct" {
		t.Fatalf("native DV5 refused: %s", w.Body.String())
	}
}

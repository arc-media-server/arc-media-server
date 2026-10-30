package api

import (
	"ams/internal/auth"
	"ams/internal/events"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestNativeCapabilitiesAndLegacyDefaults(t *testing.T) {
	v := store.Stream{Kind: "video", Codec: "h264", BitDepth: 10, Profile: "High 10", Height: 1080}
	a := store.Stream{Kind: "audio", Codec: "truehd", Channels: 8}
	f := &store.MediaFile{Container: "matroska", Streams: []store.Stream{v, a}}
	vptr, aptr := &f.Streams[0], &f.Streams[1]
	c := clientCaps{Containers: []string{"mkv", "mp4"}, Video: []string{"h264", "hevc"}, Audio: []string{"aac", "truehd"}}
	if decide(f, vptr, aptr, c).Method != "transcode" {
		t.Fatal("legacy Hi10 behavior changed")
	}
	c.MaxBitDepth = 10
	c.MaxAudioChannels = 8
	c.VideoProfiles = map[string][]string{"h264": {"High 10"}}
	if d := decide(f, vptr, aptr, c); d.Method != "direct" {
		t.Fatal(d)
	}
	c.MaxAudioChannels = 2
	if d := decide(f, vptr, aptr, c); d.Method != "remux" || !slices.Contains(d.ReasonCodes, "audio_channels") {
		t.Fatal(d)
	}
	c.MaxAudioChannels = 8
	vptr.Codec = "hevc"
	vptr.DolbyVision = true
	vptr.DolbyVisionProfile = 5
	c.HDRFormats = []string{}
	if d := decide(f, vptr, aptr, c); d.Method != "transcode" || !d.UnsupportedDV || !d.NeedsToneMap {
		t.Fatal(d)
	}
	c.HDRFormats = []string{"dolby_vision"}
	c.DolbyVisionProfiles = []int{8}
	if !decide(f, vptr, aptr, c).UnsupportedDV {
		t.Fatal("DV profile ignored")
	}
	c.DolbyVisionProfiles = []int{5}
	if d := decide(f, vptr, aptr, c); d.Method != "direct" {
		t.Fatal(d)
	}
	c.Audio = []string{"aac"}
	c.AudioPassthrough = []string{"truehd"}
	if decide(f, vptr, aptr, c).Method != "direct" {
		t.Fatal("native passthrough ignored")
	}
	c.MaxHeight = 720
	if d := decide(f, vptr, aptr, c); d.Method != "transcode" || !slices.Contains(d.ReasonCodes, "video_height") {
		t.Fatal(d)
	}
	for _, bad := range []clientCaps{{MaxBitDepth: 17}, {MaxAudioChannels: 33}, {HDRFormats: []string{"magic"}}, {DolbyVisionProfiles: []int{99}}, {Video: []string{"secret/path"}}} {
		if bad.validate() == nil {
			t.Fatal("invalid caps accepted")
		}
	}
}
func TestPlaybackPreviewNoProbeOrPlaybackMutation(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	lib, _ := s.store.SyncLibrary(ctx, "Preview", "movies", "tmdb_aired", nil)
	id, _, _ := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Preview"}, "preview")
	f := store.MediaFile{LibraryID: lib, Path: "/nonexistent/generated-fixture.mp4", Container: "mp4", DurationMS: 60000, Bitrate: 1000000, Streams: []store.Stream{{Index: 0, Kind: "video", Codec: "h264", BitDepth: 8, Height: 720}, {Index: 1, Kind: "audio", Codec: "aac", Channels: 2, IsDefault: true}}}
	fid, err := s.store.SaveFile(ctx, &f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	s.store.SetFileItems(ctx, fid, []string{id}, 0)
	s.store.CreateSession(ctx, auth.HashToken("preview"), u.ID, "password", "")
	hub := events.New()
	sc := scanner.New(ctx, s.store, probe.New("/nonexistent/fixture-ffprobe"), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
	defer sc.Wait()
	h := New(Deps{Store: s.store, Scanner: sc, Settings: s.settings, Log: s.log, Events: hub})
	call := func(action, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/items/"+id+"/"+action, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer preview")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	body := `{"caps":{"containers":["mp4"],"video":["h264"],"audio":["aac"]}}`
	_, cursor, _ := hub.Read("")
	prefsBefore, _ := s.store.UserPrefs(ctx, u.ID)
	w := call("playback-preview", body)
	var preview playbackPlan
	if err = json.Unmarshal(w.Body.Bytes(), &preview); err != nil || w.Code != 200 || preview.Method != "direct" || preview.CanStart == nil || !*preview.CanStart || !preview.DecisionOnly || preview.URL != "" || preview.SessionID != "" {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	if batch, _, _ := hub.Read(cursor); len(batch) != 0 {
		t.Fatal("preview emitted mutation")
	}
	prefsAfter, _ := s.store.UserPrefs(ctx, u.ID)
	if prefsBefore != prefsAfter {
		t.Fatal("preview changed preferences")
	}
	if data, _ := s.store.UserDataFor(ctx, u.ID, []string{id}); data[id].PositionMS != 0 || data[id].Played {
		t.Fatal("preview changed history")
	}
	w = call("playback", body)
	var started playbackPlan
	json.Unmarshal(w.Body.Bytes(), &started)
	if w.Code != 200 || started.Method != preview.Method || started.FileID != preview.FileID || started.AudioStream != preview.AudioStream || started.URL == "" {
		t.Fatal("preview/start disagree", w.Code, w.Body.String())
	}
	w = call("playback-preview", `{"caps":{"containers":["mp4"],"video":["vp9"],"audio":["aac"]}}`)
	json.Unmarshal(w.Body.Bytes(), &preview)
	if w.Code != 200 || preview.CanStart == nil || *preview.CanStart {
		t.Fatal("unplayable conversion promised", w.Body.String())
	}
	// Stored analysis remains absent: the preview cannot inspect this nonexistent fixture path.
	f.Streams = nil
	f.ID = fid
	_, err = s.store.SaveFile(ctx, &f, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	w = call("playback-preview", body)
	json.Unmarshal(w.Body.Bytes(), &preview)
	if w.Code != 200 || !preview.NeedsAnalysis || *preview.CanStart || preview.Method != "unknown" {
		t.Fatal(w.Code, w.Body.String())
	}
	known, _ := s.store.GetFileByPath(ctx, f.Path)
	if known.Probed || known.ProbeError != "" {
		t.Fatal("preview probed")
	}
	w = call("playback-preview", fmt.Sprintf(`{"file_id":%d,"caps":{"max_audio_channels":99}}`, fid))
	if w.Code != 400 {
		t.Fatal("invalid caps accepted")
	}
}

func TestPlaybackPreviewOptimizedAndQualityDecisions(t *testing.T) {
	s, u, f, id := optimizedAudioFixture(t, 60000, 500000, 720)
	request := func(preview bool, extra string) playbackPlan {
		t.Helper()
		r := httptest.NewRequest("POST", "/", strings.NewReader(fmt.Sprintf(`{"file_id":%d,"caps":{"containers":["mp4"],"video":["h264"],"audio":["aac"]}%s}`, f.ID, extra)))
		r.SetPathValue("id", id)
		r = r.WithContext(context.WithValue(r.Context(), userKey, u))
		w := httptest.NewRecorder()
		if preview {
			s.playbackPreview(w, r)
		} else {
			s.startPlayback(w, r)
		}
		var out playbackPlan
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != 200 {
			t.Fatal(w.Code, w.Body.String(), err)
		}
		return out
	}
	p := request(true, "")
	if p.Optimized == nil || p.CanStart == nil || !*p.CanStart || p.URL != "" {
		t.Fatal(p)
	}
	if len(s.playing.list()) != 0 {
		t.Fatal("preview created now-playing state")
	}
	a := request(false, "")
	if a.Method != p.Method || a.Optimized == nil || a.Optimized.SourceAudioStream != p.Optimized.SourceAudioStream {
		t.Fatal("optimized preview/start mismatch")
	}
	p = request(true, `,"max_kbps":400`)
	if p.CanStart == nil || *p.CanStart || p.Optimized != nil || !slices.Contains(p.ReasonCodes, "quality_limit") {
		t.Fatal("quality preview ignored budget", p)
	}
	p = request(true, `,"audio_stream":2`)
	if p.AudioStream != 2 || p.Optimized != nil || p.CanStart == nil || *p.CanStart {
		t.Fatal("alternate track preview selected incompatible copy", p)
	}
}

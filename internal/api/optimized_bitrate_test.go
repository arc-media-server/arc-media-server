package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"ams/internal/settings"
)

func TestOptimizedEffectiveBitrateBudgets(t *testing.T) {
	for _, tc := range []struct {
		name                                            string
		bitrate, duration                               int64
		quality, manual, remote, upload, height, others int
		away, wantCopy                                  bool
	}{
		{name: "unlimited unknown is retained", quality: 720, wantCopy: true},
		{name: "manual unknown rejected", quality: 720, manual: 2000},
		{name: "unknown duration rejected", quality: 720, manual: 2000, bitrate: 1500000},
		{name: "manual over budget", quality: 720, manual: 2000, bitrate: 4560009, duration: 6000},
		{name: "manual exact budget", quality: 720, manual: 2000, bitrate: 2000000, duration: 6000, wantCopy: true},
		{name: "one bit over budget", quality: 720, manual: 2000, bitrate: 2000001, duration: 6000},
		{name: "height limit independent of low bitrate", quality: 1080, manual: 2000, bitrate: 1500000, duration: 6000},
		{name: "client height cap", quality: 720, height: 480, bitrate: 1500000, duration: 6000},
		{name: "remote limit overrides original", quality: 720, away: true, remote: 2000, bitrate: 4560009, duration: 6000},
		{name: "remote limit tightens larger manual", quality: 720, away: true, manual: 8000, remote: 2000, bitrate: 3000000, duration: 6000},
		{name: "manual tighter than remote", quality: 720, away: true, manual: 2000, remote: 8000, bitrate: 3000000, duration: 6000},
		{name: "manual tighter than independent ceiling", quality: 720, away: true, manual: 2000, remote: 8000, bitrate: 1500000, duration: 6000, wantCopy: true},
		{name: "remote under budget", quality: 720, away: true, remote: 2000, bitrate: 1500000, duration: 6000, wantCopy: true},
		{name: "upload share over budget", quality: 720, away: true, upload: 5000, others: 1, bitrate: 3000000, duration: 6000},
		{name: "upload share under budget", quality: 720, away: true, upload: 5000, others: 1, bitrate: 1500000, duration: 6000, wantCopy: true},
		{name: "home ignores remote setting", quality: 720, remote: 2000, bitrate: 4560009, duration: 6000, wantCopy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, viewer, f, item := optimizedAudioFixture(t, tc.duration, tc.bitrate, tc.quality)
			s.settings = func() settings.Settings { return settings.Settings{RemoteKbps: tc.remote, UploadKbps: tc.upload} }
			for i := 0; i < tc.others; i++ {
				other := httptest.NewRequest("POST", "/", nil)
				other.RemoteAddr = fmt.Sprintf("198.51.100.%d:5000", i+1)
				s.playing.touch(other, fmt.Sprint(i), "")
				s.playing.mu.Lock()
				for _, p := range s.playing.byKey {
					p.address = other.RemoteAddr[:strings.LastIndex(other.RemoteAddr, ":")]
				}
				s.playing.mu.Unlock()
			}
			r := httptest.NewRequest("POST", "/", strings.NewReader(fmt.Sprintf(`{"file_id":%d,"max_kbps":%d,"caps":{"containers":["mp4"],"video":["h264"],"audio":["aac"],"max_height":%d}}`, f.ID, tc.manual, tc.height)))
			r.RemoteAddr = "127.0.0.1:4000"
			if tc.away {
				r.RemoteAddr = "203.0.113.1:4000"
			}
			r.SetPathValue("id", item)
			r = r.WithContext(context.WithValue(r.Context(), userKey, viewer))
			w := httptest.NewRecorder()
			s.startPlayback(w, r)
			if !tc.wantCopy {
				if w.Code != 503 {
					t.Fatalf("over-budget copy selected: %d %s", w.Code, w.Body.String())
				}
				return
			}
			if w.Code != 200 {
				t.Fatalf("eligible copy rejected: %d %s", w.Code, w.Body.String())
			}
			var plan playbackPlan
			if err := json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
				t.Fatal(err)
			}
			if plan.Optimized == nil || plan.Optimized.Bitrate != tc.bitrate || plan.Optimized.DurationMS != tc.duration {
				t.Fatalf("wrong served metadata: %+v", plan)
			}
			wantCeiling := s.remoteLimit(r, item)
			if plan.ServerLimitKbps != wantCeiling {
				t.Fatalf("server ceiling=%d want independent policy %d (manual %d)", plan.ServerLimitKbps, wantCeiling, tc.manual)
			}
		})
	}
}

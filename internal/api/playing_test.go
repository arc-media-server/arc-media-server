package api

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNowPlayingStop(t *testing.T) {
	n := newNowPlaying()
	r := httptest.NewRequest("GET", "/", nil)
	r = r.WithContext(context.WithValue(r.Context(), sessionKey, "tv"))

	stop, done, refused := n.openStream(r, "movie1", "")
	if refused != "" {
		t.Fatal(refused)
	}
	if why, _ := n.progress(r, "movie1", 5000, 60000, "playing"); why != "" {
		t.Fatal(why)
	}
	list := n.list()
	if len(list) != 1 || list[0].positionMS != 5000 || !list[0].streaming() {
		t.Fatalf("list = %+v", list)
	}

	if found, _ := n.stop(list[0].id, "Bedtime!"); !found {
		t.Fatal("stop found nothing")
	}
	select {
	case <-stop:
	default:
		t.Error("the open stream wasn't cut off")
	}
	done()
	if len(n.list()) != 0 {
		t.Error("a stopped playback is still listed")
	}
	if _, _, why := n.openStream(r, "movie1", ""); why != "Bedtime!" {
		t.Errorf("reconnecting: %q", why)
	}
	if why, _ := n.progress(r, "movie1", 6000, 60000, "playing"); why != "Bedtime!" {
		t.Errorf("progress after stop: %q", why)
	}
	// Something else on the same device is fine.
	if _, _, why := n.openStream(r, "movie2", ""); why != "" {
		t.Errorf("another title: %q", why)
	}

	// Closing the player ends a playback straight away.
	n.progress(r, "movie3", 1000, 60000, "playing")
	n.progress(r, "movie3", 2000, 60000, "stopped")
	for _, p := range n.list() {
		if p.itemID == "movie3" {
			t.Error("closed player still listed")
		}
	}
}

func TestDeviceName(t *testing.T) {
	for ua, want := range map[string]string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36 Edg/140.0":                   "Edge on Windows",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1": "Safari on iPhone",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:130.0) Gecko/20100101 Firefox/130.0":                                                    "Firefox on Mac",
		"curl/8.4.0": "curl/8.4.0",
		"":           "Unknown device",
	} {
		if got := deviceName(ua); got != want {
			t.Errorf("deviceName(%q) = %q, want %q", ua, got, want)
		}
	}
}

func TestNowPlayingWatchTime(t *testing.T) {
	n := newNowPlaying()
	r := httptest.NewRequest("GET", "/", nil)
	r = r.WithContext(context.WithValue(r.Context(), sessionKey, "tv"))
	key := playbackKey(r, "movie1", "")
	// report pretends the last report was ago, then reports.
	report := func(ago time.Duration, state string, pos int64) *playUpdate {
		if p := n.byKey[key]; p != nil {
			p.counted = time.Now().Add(-ago)
		}
		_, u := n.progress(r, "movie1", pos, 100_000, state)
		return u
	}

	if u := report(0, "playing", 0); u != nil {
		t.Fatal("recorded before a minute of watching")
	}
	report(20*time.Second, "playing", 20_000)
	report(time.Hour, "paused", 50_000) // a long gap counts as 30 seconds
	if u := report(10*time.Minute, "playing", 50_000); u != nil {
		t.Fatalf("paused time counted: %+v", u)
	}
	u := report(15*time.Second, "playing", 65_000)
	if u == nil || !u.add || u.watchedMS < 65_000 || u.watchedMS > 66_000 {
		t.Fatalf("after 65 seconds: %+v", u)
	}
	if again := report(time.Second, "playing", 66_000); again != nil {
		t.Fatal("added twice while the first row was being written")
	}
	n.played(key, 7)
	u = report(10*time.Second, "stopped", 95_000)
	if u == nil || u.add || u.historyID != 7 || !u.completed {
		t.Fatalf("on close: %+v", u)
	}
}

package markers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"ams/internal/store"
)

func TestFits(t *testing.T) {
	const dur = 44 * 60_000 // a 44-minute episode
	in := []store.Marker{
		{Kind: "intro", StartMS: 60_000, EndMS: 120_000},
		{Kind: "recap", StartMS: 0, EndMS: 50_000},
		{Kind: "credits", StartMS: dur - 60_000, EndMS: dur + 2_000}, // slightly past the end: trimmed
	}
	if got := fits(in, dur, 0); len(got) != 3 || got[2].EndMS != dur {
		t.Errorf("good markers = %+v", got)
	}
	bad := []store.Marker{
		{Kind: "intro", StartMS: 30 * 60_000, EndMS: 31 * 60_000},     // intro late in the episode
		{Kind: "intro", StartMS: 0, EndMS: 2_000},                     // too short
		{Kind: "credits", StartMS: 60_000, EndMS: 120_000},            // credits at the start
		{Kind: "credits", StartMS: dur - 60_000, EndMS: dur + 60_000}, // well past the end
	}
	if got := fits(bad, dur, 0); len(got) != 0 {
		t.Errorf("bad markers kept: %+v", got)
	}
	// Timed on a version 3 minutes longer: not this cut.
	if got := fits(in, dur, dur+180_000); got != nil {
		t.Errorf("another cut's markers kept: %+v", got)
	}
}

func TestFromChapters(t *testing.T) {
	got := fromChapters([]store.Chapter{
		{StartMS: 0, EndMS: 90_000, Title: "Opening"},
		{StartMS: 90_000, EndMS: 1_300_000, Title: "Part A"},
		{StartMS: 1_300_000, EndMS: 1_390_000, Title: "Ending"},
		{StartMS: 1_390_000, EndMS: 1_420_000, Title: "Preview"},
	})
	if len(got) != 2 || got[0].Kind != "intro" || got[1].Kind != "credits" || got[1].StartMS != 1_300_000 {
		t.Errorf("chapters = %+v", got)
	}
}

func TestOnline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/segments":
			if r.URL.Query().Get("imdb_id") == "tt0303461" {
				w.Write([]byte(`{"intro":{"start_ms":60000,"end_ms":110000},"recap":null,"outro":{"start_ms":2500000,"end_ms":2580000}}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case "/media":
			if r.Header.Get("Authorization") != "Bearer k" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Write([]byte(`{"group_duration_ms":2640000,"intro":[{"start_ms":null,"end_ms":45000}],"credits":[{"start_ms":2580000,"end_ms":null}]}`))
		}
	}))
	defer srv.Close()
	t.Setenv("AMS_INTRODB_URL", srv.URL)
	t.Setenv("AMS_THEINTRODB_URL", srv.URL)
	ctx := context.Background()
	ms, err := introDB(ctx, srv.Client(), "tt0303461", 1, 1)
	if err != nil || len(ms) != 2 {
		t.Fatalf("IntroDB = %+v, %v", ms, err)
	}
	if ms, _ := introDB(ctx, srv.Client(), "tt0000000", 1, 1); ms != nil {
		t.Errorf("unknown show = %+v", ms)
	}
	ms, dur, err := theIntroDB(ctx, srv.Client(), "k", 78874, 0, "", 1, 1)
	if err != nil || dur != 2_640_000 || len(ms) != 2 {
		t.Fatalf("TheIntroDB = %+v, %d, %v", ms, dur, err)
	}
	for _, m := range ms {
		if m.Kind == "intro" && m.StartMS != 0 || m.Kind == "credits" && m.EndMS != 2_640_000 {
			t.Errorf("open-ended times not filled in: %+v", m)
		}
	}
}

// TestDetect makes three short "episodes" that share an intro and credits tune between parts of
// their own, and checks both are found.
func TestDetect(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("needs ffmpeg")
	}
	dir := t.TempDir()
	// The shared tunes: chords that change every half second, so they have a fingerprint.
	intro := "aevalsrc='0.3*sin(2*PI*(220+110*floor(mod(2*t,4)))*t)+0.2*sin(2*PI*(330+55*floor(mod(3*t,5)))*t)':s=22050:d=30"
	credits := "aevalsrc='0.3*sin(2*PI*(440-60*floor(mod(2*t,6)))*t)+0.2*sin(2*PI*(523+40*floor(mod(t,3)))*t)':s=22050:d=20"
	var files []store.MarkerJob
	for ep := 1; ep <= 3; ep++ {
		path := filepath.Join(dir, "ep"+strconv.Itoa(ep)+".m4a")
		// Each episode's own parts: tunes of their own, different in every episode.
		own := func(seed, secs int) string {
			f := strconv.Itoa(150 + seed*37)
			return "aevalsrc='0.3*sin(2*PI*(" + f + "+" + strconv.Itoa(seed*13%90+20) + "*floor(mod(1.7*t,7)))*t)':s=22050:d=" + strconv.Itoa(secs)
		}
		// own 10s, intro 30s, own 120s, credits 20s, own 5s (a preview after the credits: Skip Credits
		// goes past it to the end).
		cmd := exec.Command(ffmpeg, "-v", "error", "-y",
			"-f", "lavfi", "-i", own(ep*10+1, 10), "-f", "lavfi", "-i", intro, "-f", "lavfi", "-i", own(ep*10+2, 120),
			"-f", "lavfi", "-i", credits, "-f", "lavfi", "-i", own(ep*10+3, 5),
			"-filter_complex", "[0][1][2][3][4]concat=n=5:v=0:a=1", "-c:a", "aac", path)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("making clip: %v %s", err, out)
		}
		files = append(files, store.MarkerJob{FileID: int64(ep), Path: path, DurationMS: 185_000})
	}
	found, err := detectSeason(context.Background(), ffmpeg, files)
	if err != nil {
		t.Fatal(err)
	}
	near := func(got, want int64) bool { return got > want-3000 && got < want+3000 }
	for _, f := range files {
		var intro, credits *store.Marker
		for i, m := range found[f.FileID] {
			switch m.Kind {
			case "intro":
				intro = &found[f.FileID][i]
			case "credits":
				credits = &found[f.FileID][i]
			}
		}
		if intro == nil || !near(intro.StartMS, 10_000) || !near(intro.EndMS, 40_000) {
			t.Errorf("episode %d intro = %+v", f.FileID, intro)
		}
		if credits == nil || !near(credits.StartMS, 160_000) || credits.EndMS != 185_000 {
			t.Errorf("episode %d credits = %+v", f.FileID, credits)
		}
	}
	_ = os.Remove
}

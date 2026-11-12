package optimize

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"ams/internal/events"
	"ams/internal/store"
)

// A portable stand-in process holds its completed output until the test releases it. This
// gives replacement/deletion tests an actual subprocess boundary without encoding-speed races.
func TestMain(m *testing.M) {
	if source := os.Getenv("ARC_OPTIMIZE_FAILURE_SOURCE"); source != "" {
		fmt.Fprintln(os.Stderr, "decode failed: "+source+" https://fixture.invalid/?token=secret")
		fmt.Fprintln(os.Stderr, strings.Repeat("noise", 20000))
		fmt.Fprintln(os.Stderr, "conversion failed")
		os.Exit(1)
	}
	if root := os.Getenv("ARC_OPTIMIZE_HELPER_DIR"); root != "" {
		out := os.Args[len(os.Args)-1]
		if filepath.Dir(out) != filepath.Join(root, "optimized") {
			os.Exit(2)
		}
		if err := os.WriteFile(out, []byte("generated copy"), 0600); err != nil {
			os.Exit(3)
		}
		if err := os.WriteFile(filepath.Join(root, "ready"), nil, 0600); err != nil {
			os.Exit(4)
		}
		for {
			if _, err := os.Stat(filepath.Join(root, "release")); err == nil {
				os.Exit(0)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}
	os.Exit(m.Run())
}

type conversionFixture struct {
	root string
	st   *store.Store
	m    *Manager
	f    *store.MediaFile
	item string
}

func conversionTest(t *testing.T) *conversionFixture {
	t.Helper()
	root := t.TempDir()
	st, err := store.Open(t.Context(), filepath.Join(root, "test.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	lib, err := st.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", []string{root})
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := st.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	f := &store.MediaFile{LibraryID: lib, Path: filepath.Join(root, "source.mkv"), DurationMS: 6000,
		Streams: []store.Stream{{Index: 0, Kind: "video", Codec: "h264", Width: 160, Height: 90}}}
	if err := os.WriteFile(f.Path, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	x := &conversionFixture{root: root, st: st, f: f, item: item}
	x.save(t)
	if err := st.SetFileItems(t.Context(), f.ID, []string{item}, 0); err != nil {
		t.Fatal(err)
	}
	x.m = New(filepath.Join(root, "optimized"), st, slog.New(slog.DiscardHandler))
	return x
}

func (x *conversionFixture) save(t *testing.T) {
	t.Helper()
	info, err := os.Stat(x.f.Path)
	if err != nil {
		t.Fatal(err)
	}
	x.f.Size, x.f.ModTime = info.Size(), info.ModTime().Unix()
	x.f.ID, err = x.st.SaveFile(t.Context(), x.f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
}

func (x *conversionFixture) queue(t *testing.T, quality int) *store.Optimized {
	t.Helper()
	n, err := x.st.QueueOptimize(t.Context(), []int64{x.f.ID}, quality)
	if err != nil || n != 1 {
		t.Fatalf("queue %d: %v", n, err)
	}
	o, err := x.st.NextOptimize(t.Context())
	if err != nil || o == nil {
		t.Fatalf("next %+v %v", o, err)
	}
	return o
}

func (x *conversionFixture) copy(t *testing.T, quality int) store.Optimized {
	t.Helper()
	list, err := x.st.FileOptimized(t.Context(), x.f.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range list {
		if o.Quality == quality {
			return o
		}
	}
	t.Fatalf("missing copy at %dp: %+v", quality, list)
	return store.Optimized{}
}

func helperConverter(t *testing.T, x *conversionFixture) {
	t.Helper()
	t.Setenv("ARC_OPTIMIZE_HELPER_DIR", x.root)
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	x.m.FFmpeg = func() string { return bin }
}

func waitReady(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	for {
		if _, err := os.Stat(filepath.Join(root, "ready")); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("helper did not start:", ctx.Err())
		case <-time.After(2 * time.Millisecond):
		}
	}
}

func TestConversionCannotPublishChangedOrDeletedSource(t *testing.T) {
	for _, mode := range []string{"unscanned replacement", "scanned replacement and requeue", "delete and requeue"} {
		t.Run(mode, func(t *testing.T) {
			x := conversionTest(t)
			helperConverter(t, x)
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			o := x.queue(t, 720)
			result := make(chan error, 1)
			go func() { result <- x.m.convert(ctx, o, nil) }()
			waitReady(t, ctx, x.root)
			var next *store.Optimized
			if mode == "delete and requeue" {
				if err := x.m.Delete(ctx, x.f.ID, 720); err != nil {
					t.Fatal(err)
				}
				next = x.queue(t, 720)
			} else {
				if err := os.WriteFile(x.f.Path, []byte("replacement source"), 0600); err != nil {
					t.Fatal(err)
				}
				if mode == "scanned replacement and requeue" {
					x.save(t)
					next = x.queue(t, 720)
				}
			}
			if err := os.WriteFile(filepath.Join(x.root, "release"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				if mode != "delete and requeue" && err == nil {
					t.Fatal("changed source conversion reported success")
				}
			case <-ctx.Done():
				t.Fatal("worker did not finish:", ctx.Err())
			}
			if list, err := x.st.FileOptimized(ctx, x.f.ID); err != nil || len(list) != 0 {
				t.Fatalf("stale output published: %+v %v", list, err)
			}
			files, err := os.ReadDir(x.m.dir)
			if err != nil || len(files) != 0 {
				t.Fatalf("partial/stale output retained: %+v %v", files, err)
			}
			if next == nil {
				x.save(t)
				next = x.queue(t, 720)
			}
			current, err := x.st.NextOptimize(ctx)
			if err != nil || current == nil || current.JobID != next.JobID {
				t.Fatalf("old worker overwrote requeue: %+v %v", current, err)
			}
			if err := x.m.convert(ctx, next, nil); err != nil {
				t.Fatal(err)
			}
			copy := x.copy(t, 720)
			if !Usable(&copy, x.f) {
				t.Fatal("new attempt not usable")
			}
		})
	}
}

func TestOptimizedRestartPrunesOnlyOwnedOutput(t *testing.T) {
	x := conversionTest(t)
	helperConverter(t, x)
	if err := os.WriteFile(filepath.Join(x.root, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	o := x.queue(t, 720)
	if err := x.m.convert(t.Context(), o, nil); err != nil {
		t.Fatal(err)
	}
	keep := x.copy(t, 720)
	pending := x.queue(t, 1080)
	info, _ := os.Stat(x.f.Path)
	if ok, err := x.st.ClaimOptimize(t.Context(), pending, info.ModTime().UnixNano()); err != nil || !ok {
		t.Fatalf("claim %v %v", ok, err)
	}
	partial := filepath.Join(x.m.dir, key(pending.FileID, pending.Quality)+"-"+pending.JobID+".mp4.part")
	orphan := strings.TrimSuffix(partial, ".part")
	note := filepath.Join(x.m.dir, "keep-this.txt")
	for _, p := range []string{partial, orphan, note} {
		if err := os.WriteFile(p, []byte("preserved or interrupted"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	restarted := New(x.m.dir, x.st, x.m.log)
	restarted.FFmpeg = x.m.FFmpeg
	if err := x.st.ResetConverting(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := restarted.prune(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{partial, orphan} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("interrupted output retained: %s %v", p, err)
		}
	}
	for _, p := range []string{keep.Path, note} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("registered/non-Arc file removed: %s %v", p, err)
		}
	}
	next, err := x.st.NextOptimize(t.Context())
	if err != nil || next == nil || next.JobID == pending.JobID {
		t.Fatalf("restart reused attempt: %+v %v", next, err)
	}
	if err := restarted.convert(t.Context(), next, nil); err != nil {
		t.Fatal(err)
	}
	copy := x.copy(t, 1080)
	if !Usable(&copy, x.f) {
		t.Fatal("resumed copy unusable")
	}
}

func TestInterruptedOptimizationResumes(t *testing.T) {
	x := conversionTest(t)
	x.m.Events = events.New()
	_, cursor, _ := x.m.Events.Read("")
	helperConverter(t, x)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	watchdog, stop := context.WithTimeout(t.Context(), 15*time.Second)
	defer stop()
	o := x.queue(t, 720)
	result := make(chan error, 1)
	go func() { result <- x.m.convert(ctx, o, nil) }()
	waitReady(t, watchdog, x.root)
	hints, cursor, stale := x.m.Events.Read(cursor)
	if stale || len(hints) != 1 || hints[0].Topic != "optimized" || hints[0].LibraryID != x.f.LibraryID {
		t.Fatalf("claim hints: %+v", hints)
	}
	cancel()
	select {
	case err := <-result:
		if err != context.Canceled {
			t.Fatalf("shutdown result %v", err)
		}
	case <-watchdog.Done():
		t.Fatal("conversion survived shutdown")
	}
	list, err := x.st.OptimizedVersions(t.Context())
	if err != nil || len(list) != 1 || list[0].Status != "converting" {
		t.Fatalf("interrupted work not resumable: %+v %v", list, err)
	}
	hints, cursor, stale = x.m.Events.Read(cursor)
	if stale || len(hints) != 1 || hints[0].Topic != "optimized" {
		t.Fatalf("interruption hints: %+v", hints)
	}
	if err := x.st.ResetConverting(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(x.root, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	next, err := x.st.NextOptimize(t.Context())
	if err != nil || next == nil || next.JobID == o.JobID {
		t.Fatalf("bad resumed attempt %+v %v", next, err)
	}
	if err := x.m.convert(t.Context(), next, nil); err != nil {
		t.Fatal(err)
	}
	hints, _, stale = x.m.Events.Read(cursor)
	if stale || len(hints) != 2 {
		t.Fatalf("completion hints: %+v", hints)
	}
	for _, hint := range hints {
		if hint.Topic != "optimized" {
			t.Fatalf("unexpected hint: %+v", hint)
		}
	}
	copy := x.copy(t, 720)
	if !Usable(&copy, x.f) {
		t.Fatal("resumed conversion unusable")
	}
}

func TestRealOptimizedSourceRegeneration(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe not installed")
	}
	x := conversionTest(t)
	x.m.FFmpeg = func() string { return ffmpeg }
	generate := func(seconds int) {
		t.Helper()
		out, err := exec.Command(ffmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", fmt.Sprintf("testsrc2=duration=%d:size=160x90:rate=10", seconds), "-c:v", "libx264", "-preset", "ultrafast", x.f.Path).CombinedOutput()
		if err != nil {
			t.Fatalf("generate: %v %s", err, out)
		}
	}
	duration := func(path string) float64 {
		t.Helper()
		out, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", path).CombinedOutput()
		if err != nil {
			t.Fatalf("probe: %v %s", err, out)
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	generate(6)
	x.save(t)
	original, _ := os.ReadFile(x.f.Path)
	if err := x.m.convert(t.Context(), x.queue(t, 720), nil); err != nil {
		t.Fatal(err)
	}
	old := x.copy(t, 720)
	if !Usable(&old, x.f) || math.Abs(duration(old.Path)-6) > 0.2 {
		t.Fatal("initial copy invalid")
	}
	unchanged, _ := os.ReadFile(x.f.Path)
	if sha256.Sum256(original) != sha256.Sum256(unchanged) {
		t.Fatal("optimization modified source bytes")
	}
	generate(9)
	if Usable(&old, x.f) {
		t.Fatal("unscanned source replacement accepted")
	}
	x.f.DurationMS = 9000
	x.save(t)
	if err := x.m.convert(t.Context(), x.queue(t, 720), nil); err != nil {
		t.Fatal(err)
	}
	next := x.copy(t, 720)
	if next.Path == old.Path || !Usable(&next, x.f) || math.Abs(duration(next.Path)-9) > 0.2 {
		t.Fatal("replacement did not regenerate")
	}
	if _, err := os.Stat(old.Path); !os.IsNotExist(err) {
		t.Fatalf("superseded owned copy retained: %v", err)
	}
}

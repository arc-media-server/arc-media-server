package api

import (
	"archive/zip"
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"ams/internal/logbuf"
)

func TestUnfinishedLibraryRequestIsVisibleBeforeItReturns(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := logbuf.New(20)
		defer b.Close()
		s := &Server{logs: b, log: slog.New(b.Wrap(slog.NewTextHandler(io.Discard, nil)))}
		mux := http.NewServeMux()
		release, done := make(chan struct{}), make(chan struct{})
		mux.HandleFunc("GET /api/libraries", func(w http.ResponseWriter, r *http.Request) {
			requestStage(r, "reading scanner status", 7)
			<-release
			w.WriteHeader(http.StatusOK)
		})
		go func() {
			defer close(done)
			s.logRequests(mux).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/libraries?token=must-not-log", nil))
		}()
		synctest.Wait()
		time.Sleep(11 * time.Second)
		synctest.Wait()
		lines := b.Lines(slog.LevelInfo, 0)
		if len(lines) != 1 || lines[0].Message != "request still running" || !strings.Contains(lines[0].Attrs, "reading scanner status") || !strings.Contains(lines[0].Attrs, "library_id=7") || strings.Contains(lines[0].Attrs, "must-not-log") {
			t.Errorf("unfinished request warning: %+v", lines)
		}
		// Exporting diagnostics must not wait for the blocked request/scanner.
		w := httptest.NewRecorder()
		s.downloadDiagnostics(w, httptest.NewRequest("GET", "/api/diagnostics/download", nil))
		z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
		if err != nil {
			t.Error(err)
		} else {
			found := map[string]bool{}
			for _, f := range z.File {
				r, e := f.Open()
				if e != nil {
					t.Error(e)
					continue
				}
				data, _ := io.ReadAll(r)
				r.Close()
				found[f.Name] = true
				if bytes.Contains(data, []byte("must-not-log")) {
					t.Error("raw query leaked into export")
				}
				if f.Name == "requests.json" && (!bytes.Contains(data, []byte("reading scanner status")) || !bytes.Contains(data, []byte(`"library_id": 7`))) {
					t.Errorf("missing active stage: %s", data)
				}
				if f.Name == "goroutines.txt" && !bytes.Contains(data, []byte("goroutine profile:")) {
					t.Error("missing runtime call stacks")
				}
			}
			if !found["requests.json"] || !found["goroutines.txt"] {
				t.Error("missing stall snapshots")
			}
		}
		close(release)
		<-done
		synctest.Wait()
		if len(s.requests.active) != 0 {
			t.Error("completed request retained")
		}
		lines = b.Lines(slog.LevelInfo, 0)
		if len(lines) != 2 || lines[1].Message != "slow request" || !strings.Contains(lines[1].Attrs, "request_id=1") {
			t.Errorf("missing completion correlation: %+v", lines)
		}
		time.Sleep(time.Minute)
		synctest.Wait()
		if len(b.Lines(slog.LevelInfo, 0)) != 2 {
			t.Error("warning repeated after completion")
		}
	})
}

func TestRequestDiagnosticsAreBoundedAndExcludeStreams(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var d requestDiagnostics
		for i := 0; i < maxTrackedRequests+3; i++ {
			d.begin("GET", "GET /api/libraries")
		}
		if len(d.active) != maxTrackedRequests || d.omitted != 3 {
			t.Fatal("active request bound not enforced")
		}
		for i := 0; i < 5; i++ {
			if !d.allowWarning() {
				t.Fatal("warning unexpectedly suppressed")
			}
		}
		if d.allowWarning() {
			t.Fatal("warning rate limit missing")
		}
		time.Sleep(time.Minute)
		if !d.allowWarning() {
			t.Fatal("warning budget did not recover")
		}
		for _, route := range []string{"GET /api/items/{id}/stream", "GET /api/activity", "GET /api/logs", "GET /api/diagnostics/download"} {
			if trackRequest(route) {
				t.Errorf("long-lived/frequent route tracked: %s", route)
			}
		}
	})
}

func TestFastRequestDoesNotProduceStallWarnings(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := logbuf.New(5)
		defer b.Close()
		s := &Server{log: slog.New(b.Wrap(slog.NewTextHandler(io.Discard, nil)))}
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/libraries", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
		s.logRequests(mux).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/libraries", nil))
		time.Sleep(2 * requestWarningAfter)
		synctest.Wait()
		if len(b.Lines(slog.LevelInfo, 0)) != 0 || len(s.requests.active) != 0 {
			t.Fatal("fast request produced diagnostics noise")
		}
	})
}

func TestRuntimeProfileOutputIsBounded(t *testing.T) {
	w := &boundedProfile{limit: 8}
	if n, err := w.Write([]byte("some long profile")); err != nil || n != 17 || w.buf.Len() != 8 || !w.truncated {
		t.Fatalf("unbounded profile: %+v", w)
	}
	if _, err := w.Write([]byte("more")); err != nil || w.buf.Len() != 8 {
		t.Fatal("profile continued growing")
	}
}

// Compare the bounded fast-path tracking with the same middleware on an
// untracked control route; this is a local microbenchmark, not server-load proof.
func BenchmarkRequestDiagnostics(b *testing.B) {
	for _, route := range []string{"/api/libraries", "/api/control"} {
		b.Run(route, func(b *testing.B) {
			s := &Server{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			mux := http.NewServeMux()
			mux.HandleFunc("GET "+route, func(w http.ResponseWriter, r *http.Request) {
				requestStage(r, "loading libraries", 0)
				w.WriteHeader(200)
			})
			h := s.logRequests(mux)
			r := httptest.NewRequest("GET", route, nil)
			w := httptest.NewRecorder()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				h.ServeHTTP(w, r)
			}
		})
	}
}

func TestStalledWatchlistUsesRouteTemplateNotItemIDOrQuery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := logbuf.New(10)
		defer b.Close()
		s := &Server{log: slog.New(b.Wrap(slog.NewTextHandler(io.Discard, nil)))}
		mux := http.NewServeMux()
		mux.HandleFunc("PUT /api/items/{id}/watchlist", func(w http.ResponseWriter, r *http.Request) {
			requestStage(r, "saving watchlist", 4)
			time.Sleep(11 * time.Second)
			w.WriteHeader(204)
		})
		s.logRequests(mux).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("PUT", "/api/items/private-item-value/watchlist?api_key=private-query-value", nil))
		synctest.Wait()
		lines := b.Lines(slog.LevelInfo, 0)
		if len(lines) != 2 {
			t.Fatalf("expected warning and completion, got %+v", lines)
		}
		for _, line := range lines {
			if strings.Contains(line.Attrs, "private-") || !strings.Contains(line.Attrs, "/api/items/{id}/watchlist") {
				t.Errorf("unsafe route context: %+v", line)
			}
		}
	})
}

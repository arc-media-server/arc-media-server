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

func TestDiagnosticsExportAndValidation(t *testing.T) {
	s, _ := securityServer(t)
	s.logs = logbuf.New(20)
	defer s.logs.Close()
	s.log = slog.New(s.logs.Wrap(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: s.logs.Level()})))
	s.version = "test-version"
	s.revision = "fixture-revision"
	s.logs.SetSecrets("provider-secret")
	for _, body := range []string{`{}`, `{"detailed":null}`, `{"detailed":"yes"}`, `{"detailed":true,"extra":1}`} {
		w := httptest.NewRecorder()
		s.setDiagnostics(w, httptest.NewRequest("PATCH", "/api/diagnostics", strings.NewReader(body)))
		if w.Code != 400 || s.logs.Status().Detailed {
			t.Fatalf("bad input changed logging: %d %s", w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	s.setDiagnostics(w, httptest.NewRequest("PATCH", "/api/diagnostics", strings.NewReader(`{"detailed":true}`)))
	if w.Code != 200 || !s.logs.Status().Detailed {
		t.Fatal("detailed logging not enabled")
	}
	s.log.Debug("provider-secret", "token", "private-token")
	w = httptest.NewRecorder()
	s.downloadDiagnostics(w, httptest.NewRequest("GET", "/api/diagnostics/download", nil))
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(z.File) != 4 {
		t.Fatal("unexpected diagnostic contents")
	}
	for _, f := range z.File {
		r, _ := f.Open()
		data, _ := io.ReadAll(r)
		r.Close()
		if bytes.Contains(data, []byte("provider-secret")) || bytes.Contains(data, []byte("private-token")) {
			t.Fatal("export leaked credentials")
		}
		if f.Name == "manifest.json" && !bytes.Contains(data, []byte("fixture-revision")) {
			t.Fatal("missing build revision")
		}
	}
}

func TestSlowMutationsAreLoggedWithoutDebugOrRawURLs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := logbuf.New(5)
		defer b.Close()
		s := &Server{log: slog.New(b.Wrap(slog.NewTextHandler(io.Discard, nil)))}
		mux := http.NewServeMux()
		mux.HandleFunc("PUT /api/items/{id}/artwork", func(w http.ResponseWriter, r *http.Request) { time.Sleep(2100 * time.Millisecond); w.WriteHeader(200) })
		s.logRequests(mux).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("PUT", "/api/items/fixture/artwork?token=must-not-log", nil))
		lines := b.Lines(slog.LevelInfo, 0)
		if len(lines) != 1 || lines[0].Message != "slow request" || !strings.Contains(lines[0].Attrs, "/api/items/{id}/artwork") || strings.Contains(lines[0].Attrs, "must-not-log") {
			t.Fatalf("slow request context: %+v", lines)
		}
	})
}

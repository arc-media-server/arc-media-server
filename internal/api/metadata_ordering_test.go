package api

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
)

func TestMetadataOrderingErrorsAreVisibleAndRetryable(t *testing.T) {
	ctx := t.Context()
	s, user := securityServer(t)
	user.IsAdmin = true
	fixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "provider unavailable", 503) }))
	defer fixture.Close()
	transport := fixture.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", fixture.Listener.Addr().String())
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections() })
	s.scanner = scanner.New(ctx, s.store, probe.New(""), metadata.NewTMDB("fixture-only"), metadata.NewTVDB("fixture-only"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	lib, err := s.store.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: "Example", TMDBID: 303, TVDBID: 404}, "example")
	if err != nil {
		t.Fatal(err)
	}
	ordering, err := s.store.UpsertOrdering(ctx, id, "tvdb_aired", "", "TVDB aired", "")
	if err != nil {
		t.Fatal(err)
	}
	call := func(body string, handler http.HandlerFunc) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PATCH", "/api/test", strings.NewReader(body))
		r.SetPathValue("id", id)
		r = r.WithContext(context.WithValue(r.Context(), userKey, user))
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	bad := call(`{"details_provider":"tvdb","title":"Must not save"}`, s.editItem)
	if bad.Code != 400 {
		t.Fatalf("removed setting=%d %s", bad.Code, bad.Body.String())
	}
	original, err := s.store.GetItem(ctx, id)
	if err != nil || original.Title != "Example" {
		t.Fatal("rejected legacy setting mutated metadata")
	}
	body := `{"ordering_id":` + strconv.FormatInt(ordering, 10) + `}`
	w := call(body, s.setDisplayOrdering)
	if w.Code != 502 {
		t.Fatalf("mapping failure reported success: %d %s", w.Code, w.Body.String())
	}
	for attempt := 0; attempt < 2; attempt++ {
		w = call(`{"source_ordering":"tvdb_aired"}`, s.editItem)
		if w.Code != 502 {
			t.Fatalf("attempt %d bypassed failed refresh: %d %s", attempt, w.Code, w.Body.String())
		}
	}
	if st := s.scanner.Status(lib); st.Running || st.FilesSeen != 0 || st.FilesProbed != 0 {
		t.Fatalf("error caused discovery: %+v", st)
	}
}

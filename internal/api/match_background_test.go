package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"ams/internal/activity"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
)

// Fix Match for a show answers as soon as the chosen show is identified: the episode lists,
// held here until the response is checked, are fetched by a background Activity task.
func TestSetMatchAnswersBeforeEpisodesAreFetched(t *testing.T) {
	ctx := t.Context()
	s, user := securityServer(t)
	user.IsAdmin = true
	gate := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	defer release()
	fixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // a round trip
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v4/login":
			fmt.Fprint(w, `{"data":{"token":"fixture-token"}}`)
		case r.URL.Path == "/v4/series/404/extended":
			fmt.Fprint(w, `{"data":{"id":404,"name":"Right Show","year":"2020","firstAired":"2020-01-01","seasonTypes":[{"type":"official"}]}}`)
		case r.URL.Path == "/v4/series/404/translations/eng":
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasPrefix(r.URL.Path, "/v4/series/404/episodes/"):
			select {
			case <-gate:
			case <-r.Context().Done():
				return
			}
			fmt.Fprint(w, `{"data":{"episodes":[{"id":200,"name":"Pilot","seasonNumber":1,"number":1,"aired":"2020-01-01"}]},"links":{"next":null}}`)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
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
	s.activity = activity.New()
	s.scanner = scanner.New(ctx, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB("fixture-only"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.scanner.Activity = s.activity
	t.Cleanup(s.scanner.Wait)
	lib, err := s.store.SyncLibrary(ctx, "Shows", "shows", "tvdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: "Wrong Show"}, "wrong")
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("PUT", "/api/items/"+id+"/match", strings.NewReader(`{"provider":"tvdb","id":404}`))
	r.SetPathValue("id", id)
	r = r.WithContext(context.WithValue(r.Context(), userKey, user))
	w := httptest.NewRecorder()
	start := time.Now()
	s.setMatch(w, r)
	took := time.Since(start)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		Title       string `json:"title"`
		TVDBID      int64  `json:"tvdb_id"`
		MatchTaskID int64  `json:"match_task_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "Right Show" || got.TVDBID != 404 || got.MatchTaskID == 0 {
		t.Fatalf("response %s", w.Body.String())
	}
	if took > 5*time.Second {
		t.Fatalf("answering took %v", took)
	}
	running, _ := s.activity.Snapshot()
	if len(running) != 1 || running[0].ID != got.MatchTaskID || !strings.HasPrefix(running[0].Title, "Matching Right Show") {
		t.Fatalf("Activity: %+v", running)
	}

	release()
	s.scanner.Wait()
	kids, err := s.store.Children(ctx, id)
	if err != nil || len(kids) != 1 || kids[0].Title != "Pilot" {
		t.Fatalf("episodes after the background part: %+v %v", kids, err)
	}
	_, recent := s.activity.Snapshot()
	if len(recent) != 1 || recent[0].Error != "" || recent[0].ID != got.MatchTaskID {
		t.Fatalf("finished task: %+v", recent)
	}
}

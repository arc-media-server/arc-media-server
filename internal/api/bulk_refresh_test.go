package api

import (
	"ams/internal/activity"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type bulkFixture struct {
	server          *Server
	user            *store.User
	id              string
	hits            atomic.Int32
	extra, canceled chan struct{}
	release, cancel func()
	workers         sync.WaitGroup
}

func newBulkFixture(t *testing.T) *bulkFixture {
	t.Helper()
	f := &bulkFixture{extra: make(chan struct{}, 1), canceled: make(chan struct{}, 1)}
	gate := make(chan struct{})
	var once sync.Once
	f.release = func() { once.Do(func() { close(gate) }) }
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f.hits.Add(1) > 1 {
			select {
			case f.extra <- struct{}{}:
			default:
			}
		}
		select {
		case <-gate:
		case <-r.Context().Done():
			select {
			case f.canceled <- struct{}{}:
			default:
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		id, _ := strconv.Atoi(parts[len(parts)-1])
		fmt.Fprintf(w, `{"id":%d,"title":"Fixture %d","release_date":"2020-01-01"}`, id, id)
	}))
	t.Cleanup(provider.Close)
	transport := provider.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", provider.Listener.Addr().String())
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections() })
	s, u := securityServer(t)
	f.server, f.user = s, u
	s.activity = activity.New()
	life, cancel := context.WithCancel(t.Context())
	f.cancel = cancel
	s.workContext = life
	s.workers = &f.workers
	s.scanner = scanner.New(life, s.store, probe.New(""), metadata.NewTMDB("fixture-key-not-a-secret"), metadata.NewTVDB(""), s.log)
	t.Cleanup(func() { f.release(); cancel(); f.workers.Wait(); s.scanner.Wait() })
	lib, err := s.store.SyncLibrary(t.Context(), "Fixture", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	f.id, _, err = s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture", TMDBID: 101}, "tmdb:101")
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *bulkFixture) call(t *testing.T, extra string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "/", strings.NewReader(fmt.Sprintf(`{"item_ids":[%q],"refresh":true%s}`, f.id, extra)))
	r = r.WithContext(context.WithValue(r.Context(), userKey, f.user))
	w := httptest.NewRecorder()
	f.server.editItems(w, r)
	return w
}
func (f *bulkFixture) waitStarted(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for f.hits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if f.hits.Load() == 0 {
		t.Fatal("refresh did not start")
	}
}
func TestBulkRefreshCoalescesRepeatedRequests(t *testing.T) {
	f := newBulkFixture(t)
	for range 6 {
		w := f.call(t, "")
		if w.Code != 204 {
			t.Fatalf("bulk refresh rejected: %d %s", w.Code, w.Body.String())
		}
	}
	f.waitStarted(t)
	select {
	case <-f.extra:
		t.Errorf("same title refresh started repeatedly: %d requests", f.hits.Load())
	case <-time.After(250 * time.Millisecond):
	}
}
func TestBulkRefreshShutdownCancelsAndJoinsBeforeCatalogRelease(t *testing.T) {
	f := newBulkFixture(t)
	if w := f.call(t, ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	f.waitStarted(t)
	if release, ok := f.server.store.ReserveCatalogMutation(); ok {
		release()
		t.Error("inflight refresh did not protect catalog identities")
	}
	f.cancel()
	done := make(chan struct{})
	go func() { f.workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not join metadata refresh")
	}
	select {
	case <-f.canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("provider context was not canceled")
	}
	release, ok := f.server.store.ReserveCatalogMutation()
	if !ok {
		t.Fatal("joined refresh retained catalog lease")
	}
	release()
	queue := f.server.bulkRefreshWork()
	queue.mu.Lock()
	remaining := len(queue.owners)
	queue.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("shutdown retained %d reservations", remaining)
	}
}
func TestBulkRefreshCapacityRefusalDoesNotApplyEdits(t *testing.T) {
	f := newBulkFixture(t)
	ids := make([]string, maxBulkRefreshTitles)
	for i := range ids {
		ids[i] = fmt.Sprintf("pending-%d", i)
	}
	ticket, err := f.server.bulkRefreshWork().reserve(ids)
	if err != nil {
		t.Fatal(err)
	}
	defer ticket.abort()
	w := f.call(t, `,"add_labels":["must not be saved"]`)
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("full queue admitted: %d %s", w.Code, w.Body.String())
	}
	saved, err := f.server.store.ItemLabels(t.Context(), f.id)
	if err != nil || len(saved) != 0 {
		t.Fatalf("refused request edited metadata: %+v %v", saved, err)
	}
}
func TestBulkRefreshCatalogReservationRefusesBeforeEdits(t *testing.T) {
	f := newBulkFixture(t)
	release, ok := f.server.store.ReserveCatalogMutation()
	if !ok {
		t.Fatal("reservation unavailable")
	}
	defer release()
	w := f.call(t, `,"add_labels":["must not be saved"]`)
	if w.Code != 503 {
		t.Fatalf("maintenance admitted refresh: %d %s", w.Code, w.Body.String())
	}
	saved, err := f.server.store.ItemLabels(t.Context(), f.id)
	if err != nil || len(saved) != 0 {
		t.Fatalf("maintenance request edited metadata: %+v %v", saved, err)
	}
}
func TestBulkRefreshOldCompletionCannotReleaseNewReservation(t *testing.T) {
	f := newBulkFixture(t)
	q := f.server.bulkRefreshWork()
	old, err := q.reserve([]string{f.id})
	if err != nil {
		t.Fatal(err)
	}
	defer old.abort()
	q.complete(f.id, old)
	next, err := q.reserve([]string{f.id})
	if err != nil {
		t.Fatal(err)
	}
	defer next.abort()
	q.complete(f.id, old)
	q.mu.Lock()
	owner := q.owners[f.id]
	q.mu.Unlock()
	if owner != next {
		t.Fatal("late old completion cleared newer work")
	}
}
func TestBulkRefreshBodyLimit(t *testing.T) {
	f := newBulkFixture(t)
	w := f.call(t, `,"add_labels":["`+strings.Repeat("x", 3<<20)+`"]`)
	if w.Code != 413 {
		t.Fatalf("oversized body accepted: %d", w.Code)
	}
}

func TestBulkRefreshUsesOneWorkerAcrossDifferentTitles(t *testing.T) {
	f := newBulkFixture(t)
	item, err := f.server.store.GetItem(t.Context(), f.id)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := f.server.store.EnsureItem(t.Context(), &store.Item{LibraryID: item.LibraryID, Kind: "movie", Title: "Second", TMDBID: 202}, "tmdb:202")
	if err != nil {
		t.Fatal(err)
	}
	if w := f.call(t, ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	f.waitStarted(t)
	original := f.id
	f.id = second
	if w := f.call(t, ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	f.id = original
	select {
	case <-f.extra:
		t.Error("different title started another concurrent worker")
	case <-time.After(250 * time.Millisecond):
	}
}
func TestBulkRefreshAcceptedWorkOutlivesRequestCancellation(t *testing.T) {
	f := newBulkFixture(t)
	requestCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r := httptest.NewRequest("POST", "/", strings.NewReader(fmt.Sprintf(`{"item_ids":[%q],"refresh":true}`, f.id)))
	r = r.WithContext(context.WithValue(requestCtx, userKey, f.user))
	w := httptest.NewRecorder()
	f.server.editItems(w, r)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	f.waitStarted(t)
	cancel()
	select {
	case <-f.canceled:
		t.Error("accepted refresh was canceled with the HTTP request")
	case <-time.After(250 * time.Millisecond):
	}
}

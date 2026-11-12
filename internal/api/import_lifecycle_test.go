package api

import (
	"ams/internal/activity"
	"ams/internal/importer"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type heldImportSource struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int64
}

type failingImportSource struct{ heldImportSource }

func (f *failingImportSource) Entries(context.Context, importer.Account, func(string)) ([]importer.Entry, error) {
	return nil, errors.New("GET https://fixture.invalid/private?api_key=fixture-secret token=fixture-token")
}

func TestImportFailureLogsSafeOperationAndOutcome(t *testing.T) {
	s, u := securityServer(t)
	s.workContext = t.Context()
	var workers sync.WaitGroup
	s.workers = &workers
	s.activity = activity.New()
	var logs bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&logs, nil))
	session := &importSession{id: "fixture", source: "plex", src: &failingImportSource{}, stage: "choose_accounts", created: time.Now(), accounts: []importer.Account{{ID: "one", Name: "Viewer"}}}
	s.imports.add(session)
	r := httptest.NewRequest("POST", "http://arc/api/imports/fixture/run", strings.NewReader(`{"profiles":{"one":"`+u.ID+`"}}`))
	r.SetPathValue("id", session.id)
	w := httptest.NewRecorder()
	s.runImport(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	workers.Wait()
	if !strings.Contains(logs.String(), "import started") || !strings.Contains(logs.String(), "operation=read_account") || strings.Count(logs.String(), "level=WARN") != 1 {
		t.Fatal("missing safe outcome context", logs.String())
	}
	for _, secret := range []string{"fixture-secret", "fixture-token", "https://fixture.invalid", "api_key"} {
		if strings.Contains(logs.String(), secret) || strings.Contains(session.err, secret) {
			t.Fatal("import diagnostic leaked request details", secret)
		}
	}
}

func TestImportAfterRootCancellationDoesNotCreateProfiles(t *testing.T) {
	s, _ := securityServer(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	s.workContext = ctx
	session := &importSession{id: "fixture", source: "plex", src: &heldImportSource{}, stage: "choose_accounts", created: time.Now(), accounts: []importer.Account{{ID: "one", Name: "New profile"}}}
	s.imports.add(session)
	before, err := s.store.CountUsers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://arc/api/imports/fixture/run", strings.NewReader(`{"profiles":{"one":"new"}}`))
	r.SetPathValue("id", session.id)
	w := httptest.NewRecorder()
	s.runImport(w, r)
	if w.Code != 503 {
		t.Fatal(w.Code, w.Body.String())
	}
	after, err := s.store.CountUsers(t.Context())
	if err != nil || before != after {
		t.Fatal("shutdown admission changed profiles", before, after, err)
	}
}

func (f *heldImportSource) Name() string                                         { return "Fixture" }
func (f *heldImportSource) Accounts(context.Context) ([]importer.Account, error) { return nil, nil }
func (f *heldImportSource) Entries(ctx context.Context, _ importer.Account, _ func(string)) ([]importer.Entry, error) {
	f.calls.Add(1)
	select {
	case f.entered <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.release:
		return nil, nil
	}
}

func TestImportInvalidLaterMappingCreatesNoProfiles(t *testing.T) {
	s, _ := securityServer(t)
	s.activity = activity.New()
	sess := &importSession{id: "fixture", src: &heldImportSource{}, source: "plex", stage: "choose_accounts", created: time.Now(), accounts: []importer.Account{{ID: "one", Name: "New profile"}, {ID: "two", Name: "Invalid target"}}}
	s.imports.add(sess)
	before, err := s.store.CountUsers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://arc/api/imports/fixture/run", strings.NewReader(`{"profiles":{"one":"new","two":"does-not-exist"}}`))
	r.SetPathValue("id", sess.id)
	w := httptest.NewRecorder()
	s.runImport(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	after, err := s.store.CountUsers(t.Context())
	if err != nil || after != before {
		t.Fatalf("invalid mapping created profiles: before=%d after=%d (%v)", before, after, err)
	}
}

// Release all body reads together so concurrent requests compete for admission.
// No production timing hooks or provider latency are needed.
type importRunBody struct {
	body    *strings.Reader
	arrived *sync.WaitGroup
	gate    <-chan struct{}
	once    sync.Once
}

func (b *importRunBody) Read(p []byte) (int, error) {
	b.once.Do(func() { b.arrived.Done(); <-b.gate })
	return b.body.Read(p)
}
func (*importRunBody) Close() error { return nil }

func TestConcurrentImportRunHasOneOwner(t *testing.T) {
	s, u := securityServer(t)
	s.activity = activity.New()
	source := &heldImportSource{entered: make(chan struct{}, 64), release: make(chan struct{})}
	defer close(source.release)
	session := &importSession{id: "fixture", source: "plex", src: source, stage: "choose_accounts", created: time.Now(), accounts: []importer.Account{{ID: "one", Name: "Viewer"}}}
	s.imports.add(session)
	const requests = 64
	gate := make(chan struct{})
	var arrived sync.WaitGroup
	arrived.Add(requests)
	responses := make(chan int, requests)
	for range requests {
		go func() {
			body := &importRunBody{body: strings.NewReader(`{"profiles":{"one":"` + u.ID + `"}}`), arrived: &arrived, gate: gate}
			r := httptest.NewRequest("POST", "http://arc/api/imports/fixture/run", body)
			r.SetPathValue("id", session.id)
			w := httptest.NewRecorder()
			s.runImport(w, r)
			responses <- w.Code
		}()
	}
	arrived.Wait()
	close(gate)
	successes, conflicts := 0, 0
	for range requests {
		select {
		case status := <-responses:
			switch status {
			case 200:
				successes++
			case 409:
				conflicts++
			default:
				t.Errorf("unexpected response %d", status)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("admission stuck")
		}
	}
	if successes != 1 || conflicts != requests-1 {
		t.Fatalf("duplicate admission: %d successes %d conflicts", successes, conflicts)
	}
	select {
	case <-source.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("import did not run")
	}
	if source.calls.Load() != 1 {
		t.Fatal("duplicate import workers", source.calls.Load())
	}
}

func TestImportJobCancelsAndJoinsWithServerRoot(t *testing.T) {
	s, u := securityServer(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s.workContext = ctx
	var workers sync.WaitGroup
	s.workers = &workers
	s.activity = activity.New()
	var logs bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&logs, nil))
	source := &heldImportSource{entered: make(chan struct{}, 1), release: make(chan struct{})}
	session := &importSession{id: "fixture", source: "plex", src: source, stage: "choose_accounts", created: time.Now(), accounts: []importer.Account{{ID: "one", Name: "Viewer"}}}
	s.imports.add(session)
	r := httptest.NewRequest("POST", "http://arc/api/imports/fixture/run", strings.NewReader(`{"profiles":{"one":"`+u.ID+`"}}`))
	r.SetPathValue("id", session.id)
	w := httptest.NewRecorder()
	s.runImport(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	select {
	case <-source.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("source not entered")
	}
	if release, ok := s.store.ReserveCatalogMutation(); ok {
		release()
		t.Error("catalog mutation admitted while import retained identities")
	}
	cancel()
	joined := make(chan struct{})
	go func() { workers.Wait(); close(joined) }()
	select {
	case <-joined:
	case <-time.After(3 * time.Second):
		t.Fatal("import ignored server cancellation or was not joined")
	}
	if release, ok := s.store.ReserveCatalogMutation(); !ok {
		t.Fatal("finished import retained catalog lease")
	} else {
		release()
	}
	session.mu.Lock()
	stage, problem, finished := session.stage, session.err, session.created
	session.mu.Unlock()
	if stage != "failed" || problem == "" || time.Since(finished) > time.Minute {
		t.Fatal("interrupted result not retained", stage, problem, finished)
	}
	if strings.Contains(logs.String(), "level=WARN") || strings.Contains(logs.String(), "level=ERROR") || !strings.Contains(logs.String(), "import interrupted") {
		t.Fatal("cancellation diagnostics", logs.String())
	}
	if s.imports.get(session.id) != session {
		t.Fatal("completed result expired immediately")
	}
	next := httptest.NewRecorder()
	retry := httptest.NewRequest("POST", "http://arc/api/imports/fixture/run", strings.NewReader(`{"profiles":{"one":"`+u.ID+`"}}`))
	retry.SetPathValue("id", session.id)
	s.runImport(next, retry)
	if next.Code != 409 {
		t.Fatal("failed stage unexpectedly reran without new setup", next.Code)
	}
}

func TestImportDuringCatalogMutationCreatesNoProfiles(t *testing.T) {
	s, _ := securityServer(t)
	var workers sync.WaitGroup
	s.workers = &workers
	s.workContext = t.Context()
	s.activity = activity.New()
	source := &heldImportSource{entered: make(chan struct{}, 1), release: make(chan struct{})}
	defer func() { close(source.release); workers.Wait() }()
	session := &importSession{id: "fixture", source: "plex", src: source, stage: "choose_accounts", created: time.Now(), accounts: []importer.Account{{ID: "one", Name: "Would be orphaned"}}}
	s.imports.add(session)
	release, ok := s.store.ReserveCatalogMutation()
	if !ok {
		t.Fatal("reserve fixture")
	}
	defer release()
	before, err := s.store.CountUsers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://arc/api/imports/fixture/run", strings.NewReader(`{"profiles":{"one":"new"}}`))
	r.SetPathValue("id", session.id)
	w := httptest.NewRecorder()
	s.runImport(w, r)
	if w.Code != 503 || w.Header().Get("Retry-After") != "3" {
		t.Errorf("maintenance admission %d retry=%q", w.Code, w.Header().Get("Retry-After"))
	}
	after, err := s.store.CountUsers(t.Context())
	if err != nil || before != after {
		t.Errorf("maintenance admission created profiles: %d -> %d (%v)", before, after, err)
	}
}

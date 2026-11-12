package api

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type heldPrefsBody struct {
	io.Reader
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	ctx     context.Context
}

func (b *heldPrefsBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered) })
	select {
	case <-b.release:
		return b.Reader.Read(p)
	case <-b.ctx.Done():
		return 0, b.ctx.Err()
	}
}
func (b *heldPrefsBody) Close() error { return nil }

func TestConcurrentPreferencePatchesKeepBothFieldsAndDoNotLockDuringBodyRead(t *testing.T) {
	s, u := securityServer(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	body := &heldPrefsBody{Reader: strings.NewReader(`{"hide_spoilers":true}`), entered: make(chan struct{}), release: make(chan struct{}), ctx: ctx}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(body.release) }) }
	defer release()
	var workers sync.WaitGroup
	defer func() { cancel(); release(); workers.Wait() }()
	first := httptest.NewRequest("PATCH", "/api/me/prefs", body).WithContext(context.WithValue(ctx, userKey, u))
	firstResponse := httptest.NewRecorder()
	firstDone := make(chan struct{})
	workers.Go(func() { defer close(firstDone); s.updatePrefs(firstResponse, first) })
	select {
	case <-body.entered:
	case <-ctx.Done():
		t.Fatal("first request did not reach body read")
	}
	second := httptest.NewRequest("PATCH", "/api/me/prefs", strings.NewReader(`{"theme":"light"}`)).WithContext(context.WithValue(ctx, userKey, u))
	secondResponse := httptest.NewRecorder()
	secondDone := make(chan struct{})
	workers.Go(func() { defer close(secondDone); s.updatePrefs(secondResponse, second) })
	select {
	case <-secondDone:
	case <-time.After(3 * time.Second):
		release()
		<-firstDone
		<-secondDone
		t.Fatal("a slow body held up another preference write")
	}
	if secondResponse.Code != 200 {
		release()
		<-firstDone
		t.Fatal(secondResponse.Code, secondResponse.Body.String())
	}
	release()
	<-firstDone
	if firstResponse.Code != 200 {
		t.Fatal(firstResponse.Code, firstResponse.Body.String())
	}
	saved, err := s.loadPrefsFor(t.Context(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.HideSpoilers || saved.Theme != "light" {
		t.Fatalf("concurrent independent patches lost an update: hide_spoilers=%t theme=%q", saved.HideSpoilers, saved.Theme)
	}
}

func TestInvalidPreferencePatchRollsBackAndValidResetKeepsOtherFields(t *testing.T) {
	s, u := securityServer(t)
	if err := s.store.SetUserPrefs(t.Context(), u.ID, `{"theme":"light","library_order":[2,1],"hide_spoilers":false}`); err != nil {
		t.Fatal(err)
	}
	patch := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PATCH", "/api/me/prefs", strings.NewReader(body)).WithContext(context.WithValue(t.Context(), userKey, u))
		w := httptest.NewRecorder()
		s.updatePrefs(w, r)
		return w
	}
	if w := patch(`{"theme":"pink","hide_spoilers":true}`); w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved, err := s.loadPrefsFor(t.Context(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.HideSpoilers || saved.Theme != "light" {
		t.Fatal("invalid combined patch was partly saved")
	}
	if w := patch(`{"library_order":[],"show_home_hero":false}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	saved, err = s.loadPrefsFor(t.Context(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.LibraryOrder) != 0 || saved.Theme != "light" || saved.ShowHomeHero == nil || *saved.ShowHomeHero {
		t.Fatalf("reset/false or untouched field lost: %+v", saved)
	}
}

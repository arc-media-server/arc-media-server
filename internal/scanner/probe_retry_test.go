package scanner

import (
	"ams/internal/probe"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func probeRetryQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func TestUnchangedFailedInspectionDoesNotSpawnOnEveryScan(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process-count fixture; cache tests are platform independent")
	}
	ctx := context.Background()
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{"Fixture (2020).mkv": "synthetic media"})
	file, err := st.GetFileByPath(ctx, filepath.Join(root, "Fixture (2020).mkv"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.SaveFile(ctx, file, 0, 1); err != nil {
		t.Fatal(err)
	}
	control := t.TempDir()
	counter := filepath.Join(control, "attempts")
	binary := filepath.Join(control, "ffprobe-fixture")
	script := "#!/bin/sh\nprintf x >> " + probeRetryQuote(counter) + "\nprintf 'fixture inspection failure\\n' >&2\nexit 1\n"
	if err = os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	s.prober = probe.New(binary)
	now := time.Now()
	s.probeRetryNow = func() time.Time { return now }
	scan := func() {
		t.Helper()
		if !s.Start(lib) {
			t.Fatal("scan not admitted")
		}
		s.Wait()
		if s.Status(lib).Error != "" {
			t.Fatal(s.Status(lib).Error)
		}
	}
	scan()
	scan()
	attempts, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}
	if string(attempts) != "x" {
		t.Errorf("unchanged failed file launched ffprobe %d times across two scans", len(attempts))
	}
	if status := s.Status(lib); status.FilesProbed != 0 {
		t.Errorf("repeat scan still inspected failed file: %+v", status)
	}
	saved, err := st.GetFileByPath(ctx, file.Path)
	if err != nil || saved.ProbeError == "" || saved.Missing {
		t.Fatal("backoff lost saved failure/presence", saved, err)
	}
	checkAttempts := func(want int) {
		t.Helper()
		value, err := os.ReadFile(counter)
		if err != nil || len(value) != want {
			t.Fatalf("attempts=%q want %d (%v)", value, want, err)
		}
	}
	now = now.Add(16 * time.Minute)
	scan()
	checkAttempts(2)
	info, err := os.Stat(file.Path)
	if err != nil {
		t.Fatal(err)
	}
	changed := info.ModTime().Add(time.Second)
	// Simulate a changed timestamp while preserving the fixture's source-byte guard.
	if err = os.Chtimes(file.Path, changed, changed); err != nil {
		t.Fatal(err)
	}
	scan()
	checkAttempts(3)
	secondBinary := filepath.Join(control, "replacement-ffprobe")
	if err = os.WriteFile(secondBinary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	s.prober.SetBin(secondBinary)
	scan()
	checkAttempts(4)
	if !s.StartOperation(lib, "analyze") {
		t.Fatal("explicit analysis not admitted")
	}
	s.Wait()
	checkAttempts(5)
	success := `{"format":{"format_name":"matroska","duration":"60"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":160,"height":90}]}`
	successScript := "#!/bin/sh\nprintf x >> " + probeRetryQuote(counter) + "\nprintf '%s\\n' " + probeRetryQuote(success) + "\n"
	if err = os.WriteFile(secondBinary, []byte(successScript), 0700); err != nil {
		t.Fatal(err)
	}
	if !s.StartOperation(lib, "analyze") {
		t.Fatal("repaired analysis not admitted")
	}
	s.Wait()
	checkAttempts(6)
	saved, err = st.GetFileByPath(ctx, file.Path)
	if err != nil || !saved.Probed || saved.ProbeError != "" {
		t.Fatalf("manual repair did not clear failure: %+v %v", saved, err)
	}
	scan()
	checkAttempts(6)

}

func TestProbeRetryStampAndBoundedFailureMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.mkv")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s := &Scanner{prober: probe.New("fixture-bin"), probeRetryNow: func() time.Time { return now }}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	stamp, valid := s.probeStamp(path, info.Size(), info.ModTime().Unix())
	if !valid {
		t.Fatal("fixture stamp unavailable")
	}
	s.rememberProbeFailure(1, stamp)
	if !s.deferRecentProbeFailure(1, path, info.Size(), info.ModTime().Unix(), "saved failure") {
		t.Fatal("recent unchanged failure not deferred")
	}
	if s.deferRecentProbeFailure(1, path, info.Size(), info.ModTime().Unix(), "") {
		t.Fatal("restored/uninspected state deferred without saved failure")
	}
	s.rememberProbeFailure(1, stamp)
	s.prober.SetBin("replacement-bin")
	if s.deferRecentProbeFailure(1, path, info.Size(), info.ModTime().Unix(), "saved failure") {
		t.Fatal("binary path change did not retry")
	}
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			for i := range 1000 {
				s.rememberProbeFailure(int64(worker*1000+i+10), stamp)
			}
		})
	}
	workers.Wait()
	if len(s.probeFailures) != maxRecentProbeFailures {
		t.Fatalf("failure memory unbounded: %d", len(s.probeFailures))
	}
	now = now.Add(time.Hour)
	s.rememberProbeFailure(999999, stamp)
	if len(s.probeFailures) != 1 {
		t.Fatal("expired failure hints not reclaimed")
	}
}

func TestCancelledInspectionDoesNotBecomeRecentFailure(t *testing.T) {
	ctx := context.Background()
	st, s, _, root := libraryFixture(t, "movies", map[string]string{"Fixture (2020).mkv": "synthetic media"})
	file, err := st.GetFileByPath(ctx, filepath.Join(root, "Fixture (2020).mkv"))
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.saveProbed(cancelled, file, time.Now().Unix()); err != context.Canceled {
		t.Fatal(err)
	}
	if len(s.probeFailures) != 0 {
		t.Fatal("request cancellation cached as a file failure")
	}
}

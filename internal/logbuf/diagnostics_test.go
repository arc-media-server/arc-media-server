package logbuf

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestDetailedLoggingExpiresAfterRenewal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := New(20)
		defer b.Close()
		log := slog.New(b.Wrap(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: b.Level()})))
		b.SetDetailed(true)
		if until := b.Status().Until; until == nil || until.Sub(time.Now()) != 2*time.Hour {
			t.Fatal("detailed window must be two hours")
		}
		log.Debug("enabled")
		time.Sleep(119 * time.Minute)
		if !b.Status().Detailed {
			t.Fatal("logging expired before two hours")
		}
		b.SetDetailed(true)
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		if !b.Status().Detailed {
			t.Fatal("old timer disabled renewed diagnostics")
		}
		time.Sleep(118 * time.Minute)
		synctest.Wait()
		log.Debug("expired")
		log.Info("normal")
		if b.Status().Detailed || b.Status().Until != nil {
			t.Fatal("logging did not expire")
		}
		if got := b.Lines(slog.LevelDebug, 0); len(got) != 2 || got[1].Message != "normal" {
			t.Fatalf("wrong retained records: %+v", got)
		}
	})
}

func TestRetentionRotationRestartAndSecretRedaction(t *testing.T) {
	dir := t.TempDir()
	b := New(5)
	if err := b.EnableArchive(dir); err != nil {
		t.Fatal(err)
	}
	b.archive.limit = 512
	b.SetSecrets("fixture-provider-secret")
	var console bytes.Buffer
	log := slog.New(b.Wrap(slog.NewTextHandler(&console, nil))).With("api_key", "hidden-key")
	log.Info("credentials", "url", "https://hooks.slack.com/services/T/B/private-slack", "link", "/api/cast/private-link/stream", "authorization", "Bearer private-bearer")
	for i := 0; i < 30; i++ {
		log.Info(fmt.Sprintf("record %02d", i), "err", "https://user:pass@example.com/?token=private-token&api_key=fixture-provider-secret", "password", "hidden-password")
	}
	for _, secret := range []string{"fixture-provider-secret", "hidden-key", "hidden-password", "private-token", "user:pass", "private-slack", "private-link", "private-bearer"} {
		if strings.Contains(console.String(), secret) {
			t.Fatalf("console leaked %s", secret)
		}
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != archiveCount {
		t.Fatalf("rotation files: %d", len(files))
	}
	for _, f := range files {
		info, _ := f.Info()
		if info.Size() > 512 {
			t.Fatal("archive exceeded limit")
		}
		raw, _ := os.ReadFile(filepath.Join(dir, f.Name()))
		if bytes.Contains(raw, []byte("private-token")) {
			t.Fatal("disk leaked token")
		}
	}
	restarted := New(5)
	defer restarted.Close()
	if err := restarted.EnableArchive(dir); err != nil {
		t.Fatal(err)
	}
	lines := restarted.Lines(slog.LevelDebug, 0)
	if len(lines) == 0 || lines[len(lines)-1].Message != "record 29" {
		t.Fatalf("restart lost logs: %+v", lines)
	}
	oldSeq := lines[len(lines)-1].Seq
	slog.New(restarted.Wrap(slog.NewTextHandler(io.Discard, nil))).Info("after restart")
	if len(restarted.Lines(slog.LevelDebug, oldSeq)) != 1 {
		t.Fatal("sequence did not survive restart")
	}
	data, err := restarted.Export()
	if err != nil || !bytes.Contains(data, []byte("after restart")) {
		t.Fatalf("export: %s %v", data, err)
	}
}

func TestUnavailableRetentionKeepsRecentLogs(t *testing.T) {
	b := New(5)
	defer b.Close()
	p := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(p, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if b.EnableArchive(p) == nil || b.Status().StorageError == "" {
		t.Fatal("storage failure hidden")
	}
	slog.New(b.Wrap(slog.NewTextHandler(io.Discard, nil))).Error("still available")
	data, err := b.Export()
	if err != nil || !bytes.Contains(data, []byte("still available")) {
		t.Fatalf("fallback export: %s %v", data, err)
	}
}

func TestWriteFailureStillExportsNewestRecord(t *testing.T) {
	b := New(5)
	defer b.Close()
	if err := b.EnableArchive(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	// Simulate loss of the retained-log sink, while the in-memory handler is live.
	b.archive.file.Close()
	slog.New(b.Wrap(slog.NewTextHandler(io.Discard, nil))).Error("latest failure")
	if b.Status().StorageError == "" {
		t.Fatal("write failure hidden")
	}
	data, err := b.Export()
	if err != nil || !bytes.Contains(data, []byte("latest failure")) {
		t.Fatalf("latest evidence lost: %s %v", data, err)
	}
}

func TestRedactionIsStableForShortCredentials(t *testing.T) {
	b := New(5)
	b.SetSecrets("red")
	got := b.Redact(b.Redact("red"))
	if got != "[redacted]" {
		t.Fatalf("redaction rewrote its own marker: %q", got)
	}
}

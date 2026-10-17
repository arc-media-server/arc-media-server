package logbuf

import (
	"io"
	"log/slog"
	"testing"
)

func TestBufferKeepsRecentLines(t *testing.T) {
	b := New(3)
	log := slog.New(b.Wrap(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})))
	log.Debug("one")
	log.With("lib", "Movies").Info("two", "path", "C:/a b/c.mkv")
	log.Warn("three")
	log.Error("four", "err", "boom")

	lines := b.Lines(slog.LevelDebug, 0)
	if len(lines) != 3 || lines[0].Message != "two" || lines[2].Message != "four" {
		t.Fatalf("lines = %+v", lines)
	}
	if lines[0].Attrs != `lib=Movies path="C:/a b/c.mkv"` {
		t.Errorf("attrs = %q", lines[0].Attrs)
	}
	if got := b.Lines(slog.LevelWarn, 0); len(got) != 2 {
		t.Errorf("warn and above = %+v", got)
	}
	if got := b.Lines(slog.LevelDebug, lines[1].Seq); len(got) != 1 || got[0].Message != "four" {
		t.Errorf("after = %+v", got)
	}
}

// Package logbuf keeps the most recent log lines in memory, so the admin can read them in the web
// app without finding a log file.
package logbuf

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Line is one log record.
type Line struct {
	Seq     int64     `json:"seq"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	// Attrs are the record's key=value pairs, already formatted.
	Attrs string `json:"attrs,omitempty"`
}

// Buffer holds the last N lines.
type Buffer struct {
	mu           sync.Mutex
	lines        []Line
	next         int // where the next line goes once the buffer is full
	seq          int64
	size         int
	level        slog.LevelVar
	until        time.Time
	timer        *time.Timer
	archive      *archive
	archiveError string
	secrets      []string
}

func New(size int) *Buffer {
	return &Buffer{size: size, lines: make([]Line, 0, size)}
}

func (b *Buffer) add(l Line) {
	if len(l.Message) > recordLimit {
		l.Message = l.Message[:recordLimit] + " [truncated]"
	}
	if len(l.Attrs) > recordLimit {
		l.Attrs = l.Attrs[:recordLimit] + " [truncated]"
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	l.Seq = b.seq
	if b.archive != nil {
		if err := b.archive.append(l); err != nil {
			b.archiveError = err.Error()
		}
	}
	b.keep(l)
}

// keep appends an already numbered record; the caller holds mu.
func (b *Buffer) keep(l Line) {
	if len(b.lines) < b.size {
		b.lines = append(b.lines, l)
		return
	}
	b.lines[b.next] = l
	b.next = (b.next + 1) % b.size
}

// Lines returns the kept lines at or above minLevel that come after the given sequence number,
// oldest first.
func (b *Buffer) Lines(minLevel slog.Level, after int64) []Line {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Line, 0, len(b.lines))
	for i := range b.lines {
		l := b.lines[(b.next+i)%len(b.lines)]
		if l.Seq <= after {
			continue
		}
		var lv slog.Level
		if lv.UnmarshalText([]byte(l.Level)) == nil && lv < minLevel {
			continue
		}
		l.Message, l.Attrs = b.redact(l.Message), b.redact(l.Attrs)
		out = append(out, l)
	}
	return out
}

// Wrap returns a handler that writes to h and also keeps each record in the buffer.
func (b *Buffer) Wrap(h slog.Handler) slog.Handler {
	return &handler{inner: h, buf: b}
}

type handler struct {
	inner  slog.Handler
	buf    *Buffer
	prefix string // attrs from WithAttrs, already formatted
	group  string
}

func (h *handler) Enabled(ctx context.Context, l slog.Level) bool { return h.inner.Enabled(ctx, l) }

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	clean := slog.NewRecord(r.Time, r.Level, h.buf.Redact(r.Message), r.PC)
	var sb strings.Builder
	sb.WriteString(h.prefix)
	r.Attrs(func(a slog.Attr) bool {
		a = h.buf.cleanAttr(a)
		clean.AddAttrs(a)
		writeAttr(&sb, h.group, a)
		return true
	})
	h.buf.add(Line{Time: r.Time, Level: r.Level.String(), Message: clean.Message, Attrs: h.buf.Redact(strings.TrimSpace(sb.String()))})
	return h.inner.Handle(ctx, clean)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	attrs = append([]slog.Attr(nil), attrs...)
	for i := range attrs {
		attrs[i] = h.buf.cleanAttr(attrs[i])
	}
	var sb strings.Builder
	sb.WriteString(h.prefix)
	for _, a := range attrs {
		writeAttr(&sb, h.group, a)
	}
	return &handler{inner: h.inner.WithAttrs(attrs), buf: h.buf, prefix: sb.String(), group: h.group}
}

func (h *handler) WithGroup(name string) slog.Handler {
	g := name
	if h.group != "" {
		g = h.group + "." + name
	}
	return &handler{inner: h.inner.WithGroup(name), buf: h.buf, prefix: h.prefix, group: g}
}

func writeAttr(sb *strings.Builder, group string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	key := a.Key
	if group != "" {
		key = group + "." + key
	}
	if a.Value.Kind() == slog.KindGroup {
		for _, ga := range a.Value.Group() {
			writeAttr(sb, key, ga)
		}
		return
	}
	v := a.Value.String()
	if strings.ContainsAny(v, " \"=") || v == "" {
		v = fmt.Sprintf("%q", v)
	}
	fmt.Fprintf(sb, " %s=%s", key, v)
}

package transcode

import (
	"regexp"
	"strings"
	"unicode"
)

// Keep both the first cause and the final outcome without retaining unbounded process output.
const stderrLimit = 16 * 1024

// StderrBuffer retains at most 16 KiB of a child process error output, keeping its head and tail.
// Read it only after the child has exited; it has a single writer and is not synchronized.
type StderrBuffer struct{ head, tail []byte }

func (b *StderrBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if room := stderrLimit/2 - len(b.head); room > 0 {
		take := min(room, len(p))
		b.head = append(b.head, p[:take]...)
		p = p[take:]
	}
	if len(p) > 0 {
		if len(p) >= stderrLimit/2 {
			b.tail = append(b.tail[:0], p[len(p)-stderrLimit/2:]...)
		} else {
			excess := len(b.tail) + len(p) - stderrLimit/2
			if excess > 0 {
				b.tail = append(b.tail[:0], b.tail[excess:]...)
			}
			b.tail = append(b.tail, p...)
		}
	}
	return n, nil
}
func (b *StderrBuffer) String() string { return string(b.head) + string(b.tail) }

var diagnosticURL = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s]+`)

// FFmpegFailure summarizes the first cause and final outcome, redacting URLs and the source path.
func FFmpegFailure(text, source string) string {
	text = diagnosticURL.ReplaceAllString(text, "[redacted URL]")
	if source != "" {
		text = strings.ReplaceAll(text, source, "[source]")
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, line))
		if line != "" {
			if len(line) > 512 {
				line = string([]rune(line)[:min(128, len([]rune(line)))]) + "…"
			}
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return "no details"
	}
	if len(lines) == 1 || lines[0] == lines[len(lines)-1] {
		return lines[0]
	}
	return lines[0] + "; " + lines[len(lines)-1]
}

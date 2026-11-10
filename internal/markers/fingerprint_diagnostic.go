package markers

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"unicode"
)

var errNoAudio = errors.New("no usable audio fingerprint")

type fingerprintStderr struct{ bytes.Buffer }

func (b *fingerprintStderr) Write(p []byte) (int, error) {
	n := len(p)
	room := 4096 - b.Len()
	if room > 0 {
		b.Buffer.Write(p[:min(room, n)])
	}
	return n, nil
}

var fingerprintURL = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^\s]+`)

func fingerprintDetail(text, path string) string {
	text = fingerprintURL.ReplaceAllString(text, "[redacted URL]")
	text = strings.ReplaceAll(text, path, "[source]")
	text = strings.TrimSpace(text)
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
	runes := []rune(text)
	if len(runes) > 256 {
		text = string(runes[:256]) + "…"
	}
	return text
}

package logbuf

import (
	"log/slog"
	"regexp"
	"strings"
	"time"
)

const detailedDuration = 2 * time.Hour

type Status struct {
	Detailed     bool       `json:"detailed"`
	Until        *time.Time `json:"until,omitempty"`
	Persistent   bool       `json:"persistent"`
	StorageError string     `json:"storage_error,omitempty"`
}

func (b *Buffer) Level() *slog.LevelVar { return &b.level }

// SetDetailed enables extra records for two hours, with no restart or database write.
func (b *Buffer) SetDetailed(enabled bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
	b.until = time.Time{}
	b.level.Set(slog.LevelInfo)
	if !enabled {
		return
	}
	b.until = time.Now().Add(detailedDuration)
	b.level.Set(slog.LevelDebug)
	expires := b.until
	b.timer = time.AfterFunc(detailedDuration, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.until.Equal(expires) {
			b.level.Set(slog.LevelInfo)
			b.until = time.Time{}
		}
	})
}

func (b *Buffer) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := Status{Detailed: b.level.Level() <= slog.LevelDebug, Persistent: b.archive != nil, StorageError: b.redact(b.archiveError)}
	if !b.until.IsZero() {
		until := b.until
		s.Until = &until
	}
	return s
}

func (b *Buffer) SetSecrets(values ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Keep prior values too: old provider errors may still be retained after a key changes.
	for _, v := range values {
		if v != "" {
			found := false
			for _, old := range b.secrets {
				if v == old {
					found = true
					break
				}
			}
			if !found {
				b.secrets = append(b.secrets, v)
			}
		}
	}
}

var credentialQuery = regexp.MustCompile(`(?i)(api[_-]?key|access[_-]?token|token|password|secret)=([^&\s"']+)`)
var bearer = regexp.MustCompile(`(?i)\bBearer\s+[a-z0-9._~+/=-]+`)
var userInfo = regexp.MustCompile(`(?i)(https?://)[^/\s@]+@`)
var webhookSecret = regexp.MustCompile(`(?i)(https?://(?:hooks\.slack\.com/services|(?:discord(?:app)?\.com)/api/webhooks)/)[^\s"']+`)
var linkSecret = regexp.MustCompile(`(?i)(/api/(?:invites|auth/link|cast)/)[^/\s?"']+`)

func (b *Buffer) redact(value string) string {
	for _, secret := range b.secrets {
		parts := strings.Split(value, "[redacted]")
		for i := range parts {
			parts[i] = strings.ReplaceAll(parts[i], secret, "[redacted]")
		}
		value = strings.Join(parts, "[redacted]")
	}
	value = credentialQuery.ReplaceAllString(value, "$1=[redacted]")
	value = bearer.ReplaceAllString(value, "Bearer [redacted]")
	value = webhookSecret.ReplaceAllString(value, "$1[redacted]")
	value = linkSecret.ReplaceAllString(value, "$1[redacted]")
	return userInfo.ReplaceAllString(value, "$1[redacted]@")
}

func (b *Buffer) Redact(value string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.redact(value)
}

func (b *Buffer) cleanAttr(a slog.Attr) slog.Attr {
	a.Value = a.Value.Resolve()
	key := strings.ToLower(strings.ReplaceAll(a.Key, "-", "_"))
	if strings.Contains(key, "password") || strings.Contains(key, "token") || strings.Contains(key, "secret") || strings.Contains(key, "api_key") || strings.Contains(key, "apikey") || key == "authorization" || key == "pin" || key == "key" {
		a.Value = slog.StringValue("[redacted]")
	} else if a.Value.Kind() == slog.KindGroup {
		group := append([]slog.Attr(nil), a.Value.Group()...)
		for i := range group {
			group[i] = b.cleanAttr(group[i])
		}
		a.Value = slog.GroupValue(group...)
	} else if a.Value.Kind() == slog.KindString || a.Value.Kind() == slog.KindAny {
		a.Value = slog.StringValue(b.Redact(a.Value.String()))
	}
	return a
}

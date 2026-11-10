package api

import (
	"ams/internal/settings"
	"log/slog"
	"net/http"
	"reflect"
	"slices"
	"strings"
)

// Log compiled setting names only, after a successful commit. Request keys and
// values are untrusted and never used as diagnostic text, even for unknown fields.
func (s *Server) logCommittedSettings(r *http.Request, before, after settings.Settings) {
	if !s.log.Enabled(r.Context(), slog.LevelInfo) {
		return
	}
	old, next := reflect.ValueOf(before), reflect.ValueOf(after)
	names := []string{}
	for i := 0; i < old.NumField(); i++ {
		field := old.Type().Field(i)
		if !field.IsExported() || field.Name == "BackgroundEpoch" {
			continue
		}
		if !reflect.DeepEqual(old.Field(i).Interface(), next.Field(i).Interface()) {
			names = append(names, field.Name)
		}
	}
	if len(names) == 0 {
		return
	}
	slices.Sort(names)
	s.log.InfoContext(r.Context(), "settings changed", "by", currentUserID(r), "settings", strings.Join(names, ", "))
}

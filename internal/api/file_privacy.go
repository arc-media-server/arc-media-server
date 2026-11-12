package api

import (
	"ams/internal/scanner"
	"ams/internal/store"
	"net/http"
	"path"
	"strings"
)

func adminFileDetails(r *http.Request) bool {
	user := currentUser(r)
	return user != nil && user.IsAdmin
}
func sourceFilename(source string) string {
	if source == "" {
		return ""
	}
	normalized := strings.ReplaceAll(source, `\`, "/")
	if len(normalized) > 1 && normalized[1] == ':' {
		normalized = normalized[2:]
	}
	return path.Base(normalized)
}

// Only response copies change. Streaming, probing and server-side folder navigation retain the
// original store paths; raw filesystem/probe diagnostics belong to the administrator.
func profileMediaFiles(r *http.Request, files []store.MediaFile) []store.MediaFile {
	if adminFileDetails(r) {
		return files
	}
	out := append([]store.MediaFile(nil), files...)
	for i := range out {
		out[i].Path = sourceFilename(out[i].Path)
		if out[i].ProbeError != "" {
			out[i].ProbeError = "File details could not be read; ask an administrator"
		}
	}
	return out
}
func profileScanStatus(r *http.Request, status scanner.Status) scanner.Status {
	if !adminFileDetails(r) && status.Error != "" {
		status.Error = "Library work could not finish; ask an administrator"
	}
	return status
}

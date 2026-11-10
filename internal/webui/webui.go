// Package webui serves the web app built from /web, embedded into the binary.
package webui

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

func init() {
	// For "Add to Home Screen": the app's manifest.
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

//go:embed all:dist
var dist embed.FS

// The production app/docs bundle executes only its own scripts and the opt-in Cast SDK.
// React/Scalar styles need inline style attributes; media/artwork/connection schemes preserve
// provider images, native media and casting. This is browser hardening, not a DOM-XSS audit.
const contentPolicy = "default-src 'self'; script-src 'self' https://www.gstatic.com; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: http: https:; media-src 'self' blob: http: https:; connect-src 'self' http: https: ws: wss:; font-src 'self' data:; worker-src 'self' blob:; frame-src 'self' https://*.google.com https://*.gstatic.com; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

// Handler serves the built web app. Unknown paths get index.html so client-side routes like
// /series/abc work when loaded directly. Built assets have hashed names and are cached forever.
func Handler() http.Handler {
	files, _ := fs.Sub(dist, "dist")
	if _, err := fs.Stat(files, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("The web app hasn't been built. Run `npm install && npm run build` in the web folder, then rebuild AMS.\n"))
		})
	}
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", contentPolicy)
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" || p == "." {
			p = "index.html"
		}
		if _, err := fs.Stat(files, p); err != nil {
			p = "index.html"
		}
		if p == "index.html" {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, files, "index.html")
			return
		}
		if strings.HasPrefix(p, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else if p == "docs" {
			// The API documentation page (docs/index.html), like index.html never cached.
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}

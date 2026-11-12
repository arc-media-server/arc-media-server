package api

import (
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"ams/internal/artwork"
)

// Pre-roll videos play before movies, as in Plex: the files (or folders of them) chosen in
// Settings, all in order or one at random.

var prerollExts = []string{".mp4", ".m4v", ".mov", ".webm", ".mkv"}

// A pre-roll is an explicitly selected regular video file, never an arbitrary config file,
// pipe/device or file symlink. Folder aliases remain supported; use a linked video's real path.
func regularPrerollFile(path string) bool {
	if !slices.Contains(prerollExts, strings.ToLower(filepath.Ext(path))) {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

// prerollFiles lists the pre-roll videos: each listed file, and the videos in each listed folder.
func prerollFiles(paths string) []string {
	var out []string
	for _, line := range strings.FieldsFunc(paths, func(r rune) bool { return r == '\n' || r == ';' }) {
		p := strings.Trim(strings.TrimSpace(line), `"`)
		if p == "" {
			continue
		}
		info, err := os.Stat(p)
		switch {
		case err != nil:
			continue
		case !info.IsDir():
			if regularPrerollFile(p) {
				out = append(out, p)
			}
		default:
			entries, err := os.ReadDir(p)
			if err != nil {
				continue
			}
			for _, e := range entries {
				candidate := filepath.Join(p, e.Name())
				if regularPrerollFile(candidate) {
					out = append(out, candidate)
				}
			}
		}
	}
	return out
}

// preroll: GET /api/items/{id}/preroll lists the pre-roll videos to play before a movie (none for
// episodes and home videos, or when none are set).
func (s *Server) preroll(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	type clip struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	}
	out := []clip{}
	st := s.settings()
	files := prerollFiles(st.PrerollPaths)
	if it.Kind != "movie" || it.MetadataState == "locked" || len(files) == 0 {
		writeJSON(w, http.StatusOK, out)
		return
	}
	picked := make([]int, len(files))
	for i := range picked {
		picked[i] = i
	}
	if st.PrerollMode != "all" {
		picked = []int{rand.IntN(len(files))}
	}
	for _, i := range picked {
		out = append(out, clip{Title: "Pre-roll", URL: "/api/preroll/" + strconv.Itoa(i) + "?v=" + artwork.Key(files[i])})
	}
	writeJSON(w, http.StatusOK, out)
}

// prerollVideo: GET /api/preroll/{n} plays one of the pre-roll videos.
func (s *Server) prerollVideo(w http.ResponseWriter, r *http.Request) {
	files := prerollFiles(s.settings().PrerollPaths)
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 0 || n >= len(files) {
		writeError(w, http.StatusNotFound, "no such pre-roll video")
		return
	}
	http.ServeFile(w, r, files[n])
}

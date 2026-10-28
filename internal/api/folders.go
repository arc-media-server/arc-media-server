package api

import (
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// folderDTO is a folder inside an Other videos library.
type folderDTO struct {
	Name string `json:"name"`
	// Where it is, for ?path= ("<root>/<folder>/<folder>", root being the library folder's number).
	Path string `json:"path"`
	// Videos in it and every folder below it.
	Count int `json:"count"`
	// A video in it whose picture stands for the folder, and that picture.
	CoverID string `json:"cover_id,omitempty"`
	Cover   string `json:"cover,omitempty"`
}

// libraryFolders: GET /api/libraries/{id}/folders?path= browses an Other videos library folder by
// folder: the folders inside path (none: the top) and the videos directly in it, oldest first.
// A library with several folders starts with one entry per folder.
func (s *Server) libraryFolders(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	if lib.Kind != "videos" {
		writeError(w, http.StatusBadRequest, "folders are for Other videos libraries")
		return
	}
	videos, err := s.store.LibraryVideos(r.Context(), lib.ID, access(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	// Each video's place: its library folder's number, then the folders below it.
	type placed struct {
		segments []string
		index    int
	}
	where := make([]placed, 0, len(videos))
	for i, v := range videos {
		for root, dir := range lib.Paths {
			rel, err := filepath.Rel(dir, filepath.Dir(v.Path))
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			segs := []string{strconv.Itoa(root)}
			if rel != "." {
				segs = append(segs, strings.Split(filepath.ToSlash(rel), "/")...)
			}
			where = append(where, placed{segs, i})
			break
		}
	}
	// One library folder starts inside it; several start with one entry each.
	want := strings.Split(strings.Trim(r.URL.Query().Get("path"), "/"), "/")
	if want[0] == "" {
		want = nil
		if len(lib.Paths) == 1 {
			want = []string{"0"}
		}
	}
	folders := map[string]*folderDTO{}
	var items []int
	for _, p := range where {
		if len(p.segments) < len(want) || !equalPrefix(p.segments, want) {
			continue
		}
		if len(p.segments) == len(want) {
			items = append(items, p.index)
			continue
		}
		child := p.segments[len(want)]
		f := folders[child]
		if f == nil {
			name := child
			if len(want) == 0 { // a library folder, by its own name
				n, _ := strconv.Atoi(child)
				name = filepath.Base(lib.Paths[n])
			}
			f = &folderDTO{Name: name, Path: strings.Join(append(append([]string{}, want...), child), "/")}
			folders[child] = f
		}
		f.Count++
		if f.CoverID == "" {
			f.CoverID = videos[p.index].Item.ID
			f.Cover = toDTO(videos[p.index].Item).Images["primary"]
		}
	}
	if len(want) > 0 && len(items) == 0 && len(folders) == 0 {
		writeError(w, http.StatusNotFound, "no such folder")
		return
	}
	outFolders := make([]folderDTO, 0, len(folders))
	for _, f := range folders {
		outFolders = append(outFolders, *f)
	}
	sort.Slice(outFolders, func(i, j int) bool { return strings.ToLower(outFolders[i].Name) < strings.ToLower(outFolders[j].Name) })
	dtos := make([]itemDTO, 0, len(items))
	for _, i := range items {
		dtos = append(dtos, toDTO(videos[i].Item))
	}
	if err := s.fillUserData(r, ptrs(dtos)); err != nil {
		s.fail(w, err)
		return
	}
	// The way back up: each folder above, by name.
	type crumb struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	trail := []crumb{}
	for i := 1; i <= len(want); i++ {
		if i == 1 && len(lib.Paths) == 1 {
			continue // the library itself is the top
		}
		name := want[i-1]
		if i == 1 {
			n, _ := strconv.Atoi(name)
			if n >= 0 && n < len(lib.Paths) {
				name = filepath.Base(lib.Paths[n])
			}
		}
		trail = append(trail, crumb{Name: name, Path: strings.Join(want[:i], "/")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": strings.Join(want, "/"), "trail": trail, "folders": outFolders, "items": dtos})
}

func equalPrefix(a, prefix []string) bool {
	for i := range prefix {
		if a[i] != prefix[i] {
			return false
		}
	}
	return true
}

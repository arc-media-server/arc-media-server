// Package api serves AMS's native REST API.
package api

import (
	"ams/internal/activity"
	"ams/internal/notify"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"ams/internal/artwork"
	"ams/internal/auth"
	"ams/internal/backup"
	"ams/internal/logbuf"
	"ams/internal/metadata"
	"ams/internal/opensubtitles"
	"ams/internal/scanner"
	"ams/internal/settings"
	"ams/internal/store"
	"ams/internal/subtitles"
	"ams/internal/transcode"
	"ams/internal/trickplay"
	"ams/internal/webui"
)

// Deps is what the API needs from the rest of AMS.
type Deps struct {
	Store   *store.Store
	Scanner *scanner.Scanner
	Artwork *artwork.Cache
	Backups *backup.Manager
	// Activity tracks background work for the Activity menu.
	Activity *activity.Tracker
	// Notify sends events to webhooks.
	Notify *notify.Notifier
	// AfterRestore runs once a backup has been restored, to bring libraries back in step.
	AfterRestore func()
	// LibrariesChanged runs after a library is added, changed or removed.
	LibrariesChanged func()
	// Settings returns the current settings; ApplySettings checks, saves and applies new ones.
	Settings      func() settings.Settings
	ApplySettings func(ctx context.Context, s settings.Settings) error
	// FFprobeStatus describes the ffprobe in use, for the Settings page.
	FFprobeStatus func() any
	// Where artwork the user uploads is kept.
	UploadsDir string
	// Converts files players can't play as they are; nil if unavailable.
	Transcoder *transcode.Manager
	// Finds subtitles online.
	OpenSubtitles *opensubtitles.Client
	// Makes the timeline thumbnails; nil if unavailable.
	Trickplay *trickplay.Manager
	// Reads subtitles; nil if unavailable.
	Subtitles *subtitles.Cache
	// Recent log lines, for the dashboard.
	Logs    *logbuf.Buffer
	Log     *slog.Logger
	Version string
}

type Server struct {
	store            *store.Store
	scanner          *scanner.Scanner
	artwork          *artwork.Cache
	backups          *backup.Manager
	afterRestore     func()
	librariesChanged func()
	settings         func() settings.Settings
	applySettings    func(ctx context.Context, s settings.Settings) error
	ffprobeStatus    func() any
	uploadsDir       string
	previews         *previews
	playing          *nowPlaying
	transcoder       *transcode.Manager
	subtitles        *subtitles.Cache
	trickplay        *trickplay.Manager
	subsOnline       *opensubtitles.Client
	logs             *logbuf.Buffer
	activity         *activity.Tracker
	notify           *notify.Notifier
	imports          importSessions
	limiter          *auth.Limiter
	setupMu          sync.Mutex
	log              *slog.Logger
	version          string
}

func New(d Deps) http.Handler {
	s := &Server{store: d.Store, scanner: d.Scanner, artwork: d.Artwork, backups: d.Backups,
		afterRestore: d.AfterRestore, librariesChanged: d.LibrariesChanged, settings: d.Settings,
		applySettings: d.ApplySettings, ffprobeStatus: d.FFprobeStatus, uploadsDir: d.UploadsDir, previews: &previews{},
		playing: newNowPlaying(), transcoder: d.Transcoder, subtitles: d.Subtitles, trickplay: d.Trickplay, subsOnline: d.OpenSubtitles, logs: d.Logs, activity: d.Activity, notify: d.Notify, limiter: auth.NewLimiter(), log: d.Log, version: d.Version}
	mux := http.NewServeMux()

	// Anyone: signing in.
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/auth/status", s.authStatus)
	mux.HandleFunc("POST /api/auth/setup", s.setup)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.HandleFunc("POST /api/auth/profile", s.chooseProfile)
	mux.HandleFunc("DELETE /api/auth/profile", s.leaveProfile)
	mux.HandleFunc("POST /api/auth/logout", s.logout)

	// Every profile: watching.
	user, admin := s.signedIn, s.adminOnly
	mux.HandleFunc("GET /api/libraries", user(s.listLibraries))
	mux.HandleFunc("GET /api/libraries/{id}/scan", user(s.scanStatus))
	mux.HandleFunc("GET /api/labels", admin(s.listLabels))
	mux.HandleFunc("GET /api/activity", admin(s.getActivity))
	mux.HandleFunc("POST /api/imports", admin(s.startImport))
	mux.HandleFunc("GET /api/webhooks", admin(s.listWebhooks))
	mux.HandleFunc("POST /api/webhooks", admin(s.saveWebhook))
	mux.HandleFunc("PUT /api/webhooks/{id}", admin(s.saveWebhook))
	mux.HandleFunc("DELETE /api/webhooks/{id}", admin(s.deleteWebhook))
	mux.HandleFunc("POST /api/webhooks/{id}/test", admin(s.testWebhook))
	mux.HandleFunc("GET /api/news", user(s.news))
	mux.HandleFunc("GET /api/items/{id}/theme", user(s.theme))
	mux.HandleFunc("GET /api/items/{id}/preroll", user(s.preroll))
	mux.HandleFunc("GET /api/preroll/{n}", user(s.prerollVideo))
	mux.HandleFunc("POST /api/news/seen", user(s.newsSeen))
	mux.HandleFunc("GET /api/imports/{id}", admin(s.getImport))
	mux.HandleFunc("POST /api/imports/{id}/server", admin(s.chooseImportServer))
	mux.HandleFunc("POST /api/imports/{id}/run", admin(s.runImport))
	mux.HandleFunc("POST /api/items/bulk", admin(s.editItems))
	mux.HandleFunc("POST /api/items/merge", admin(s.mergeItems))
	mux.HandleFunc("POST /api/items/{id}/split", admin(s.splitItem))
	mux.HandleFunc("GET /api/libraries/{id}/facets", user(s.libraryFacets))
	mux.HandleFunc("GET /api/items", user(s.listItems))
	mux.HandleFunc("GET /api/items/{id}", user(s.getItem))
	mux.HandleFunc("GET /api/items/{id}/stream", user(s.stream))
	mux.HandleFunc("POST /api/items/{id}/playback", user(s.startPlayback))
	mux.HandleFunc("GET /api/items/{id}/subtitles", user(s.listSubtitles))
	mux.HandleFunc("GET /api/items/{id}/subtitles/search", user(s.searchSubtitles))
	mux.HandleFunc("POST /api/items/{id}/subtitles/download", user(s.downloadSubtitles))
	mux.HandleFunc("GET /api/items/{id}/subtitles/{track}", user(s.subtitleCues))
	mux.HandleFunc("GET /api/items/{id}/trickplay", user(s.trickplayInfo))
	mux.HandleFunc("GET /api/items/{id}/trickplay/{file}/{sheet}", user(s.trickplaySheet))
	mux.HandleFunc("GET /api/transcode/{sid}/{file}", user(s.transcodeFile))
	mux.HandleFunc("POST /api/transcode/{sid}/ping", user(s.pingTranscode))
	mux.HandleFunc("DELETE /api/transcode/{sid}", user(s.stopTranscode))
	mux.HandleFunc("GET /api/items/{id}/images/{type}", user(s.image))
	mux.HandleFunc("GET /api/people/{id}/image", user(s.personImage))
	mux.HandleFunc("GET /api/people/{id}", user(s.getPerson))
	mux.HandleFunc("GET /api/search", user(s.searchAll))
	mux.HandleFunc("GET /api/extras/{id}/stream", user(s.streamExtra))
	mux.HandleFunc("GET /api/libraries/{id}/collections", user(s.listCollections))
	mux.HandleFunc("GET /api/collections/{id}", user(s.getCollection))
	mux.HandleFunc("GET /api/collections/{id}/images/{type}", user(s.collectionImage))
	mux.HandleFunc("GET /api/series/{id}/seasons", user(s.seasons))
	mux.HandleFunc("GET /api/series/{id}/seasons/{n}/image", user(s.seasonImage))
	mux.HandleFunc("GET /api/libraries/{id}/recent-shows", user(s.recentlyAddedShows))
	mux.HandleFunc("GET /api/series/{id}/orderings", user(s.listOrderings))
	mux.HandleFunc("PUT /api/series/{id}/display-ordering", user(s.setDisplayOrdering))
	mux.HandleFunc("PUT /api/items/{id}/progress", user(s.reportProgress))
	mux.HandleFunc("PUT /api/items/{id}/played", user(s.setPlayed))
	mux.HandleFunc("GET /api/items/{id}/next", user(s.nextEpisode))
	mux.HandleFunc("GET /api/items/{id}/similar", user(s.similar))
	mux.HandleFunc("GET /api/libraries/{id}/recommended", user(s.recommended))
	mux.HandleFunc("PUT /api/items/{id}/rating", user(s.setUserRating))
	mux.HandleFunc("PUT /api/items/{id}/watchlist", user(s.setWatchlist))
	mux.HandleFunc("GET /api/watchlist", user(s.watchlist))
	mux.HandleFunc("GET /api/home/continue", user(s.continueWatching))
	mux.HandleFunc("GET /api/home/next-up", user(s.nextUp))
	mux.HandleFunc("DELETE /api/home/continue/{id}", user(s.removeFromContinue))
	mux.HandleFunc("GET /api/playlists", user(s.listPlaylists))
	mux.HandleFunc("POST /api/playlists", user(s.createPlaylist))
	mux.HandleFunc("GET /api/playlists/{id}", user(s.getPlaylist))
	mux.HandleFunc("PATCH /api/playlists/{id}", user(s.updatePlaylist))
	mux.HandleFunc("DELETE /api/playlists/{id}", user(s.deletePlaylist))
	mux.HandleFunc("POST /api/playlists/{id}/items", user(s.playlistAdd))
	mux.HandleFunc("DELETE /api/playlists/{id}/entries/{entry}", user(s.playlistRemove))
	mux.HandleFunc("PUT /api/playlists/{id}/order", user(s.playlistOrder))
	mux.HandleFunc("GET /api/me/year", user(s.myYear))
	mux.HandleFunc("GET /api/me/prefs", user(s.getPrefs))
	mux.HandleFunc("PATCH /api/me/prefs", user(s.updatePrefs))

	// The admin profile: running the server.
	mux.HandleFunc("POST /api/auth/password", admin(s.changePassword))
	mux.HandleFunc("GET /api/profiles", admin(s.listProfiles))
	mux.HandleFunc("POST /api/profiles", admin(s.createProfile))
	mux.HandleFunc("PATCH /api/profiles/{id}", admin(s.updateProfile))
	mux.HandleFunc("GET /api/profiles/{id}/prefs", admin(s.getProfilePrefs))
	mux.HandleFunc("PATCH /api/profiles/{id}/prefs", admin(s.updateProfilePrefs))
	mux.HandleFunc("GET /api/home/default", admin(s.getDefaultHome))
	mux.HandleFunc("PUT /api/home/default", admin(s.setDefaultHome))
	mux.HandleFunc("DELETE /api/profiles/{id}", admin(s.deleteProfile))
	mux.HandleFunc("GET /api/playing", admin(s.listPlaying))
	mux.HandleFunc("POST /api/playing/{id}/stop", admin(s.stopPlaying))
	mux.HandleFunc("GET /api/logs", admin(s.serverLog))
	mux.HandleFunc("GET /api/stats", admin(s.statsOverview))
	mux.HandleFunc("GET /api/stats/history", admin(s.statsHistory))
	mux.HandleFunc("GET /api/stats/storage", admin(s.statsStorage))
	mux.HandleFunc("GET /api/settings", admin(s.getSettings))
	mux.HandleFunc("PATCH /api/settings", admin(s.updateSettings))
	mux.HandleFunc("POST /api/libraries", admin(s.createLibrary))
	mux.HandleFunc("PATCH /api/libraries/{id}", admin(s.updateLibrary))
	mux.HandleFunc("DELETE /api/libraries/{id}", admin(s.deleteLibrary))
	mux.HandleFunc("POST /api/libraries/{id}/scan", admin(s.startScan))
	mux.HandleFunc("GET /api/libraries/{id}/unmatched", admin(s.unmatched))
	mux.HandleFunc("GET /api/libraries/{id}/health", admin(s.libraryHealth))
	mux.HandleFunc("POST /api/collections", admin(s.createCollection))
	mux.HandleFunc("PATCH /api/collections/{id}", admin(s.updateCollection))
	mux.HandleFunc("DELETE /api/collections/{id}", admin(s.deleteCollection))
	mux.HandleFunc("GET /api/collections/{id}/artwork", admin(s.listCollectionArtwork))
	mux.HandleFunc("PUT /api/collections/{id}/artwork", admin(s.chooseCollectionArtwork))
	mux.HandleFunc("POST /api/collections/{id}/artwork/{type}", admin(s.uploadCollectionArtwork))
	mux.HandleFunc("POST /api/collections/{id}/items", admin(s.collectionAddItems))
	mux.HandleFunc("DELETE /api/collections/{id}/items/{item}", admin(s.collectionRemoveItem))
	mux.HandleFunc("PUT /api/collections/{id}/order", admin(s.orderCollection))
	mux.HandleFunc("POST /api/libraries/{id}/compatibility", admin(s.compatibility))
	mux.HandleFunc("POST /api/libraries/{id}/empty-trash", admin(s.emptyTrash))
	mux.HandleFunc("DELETE /api/items/{id}", admin(s.removeItem))
	mux.HandleFunc("GET /api/health/libraries", admin(s.healthSummary))
	mux.HandleFunc("POST /api/library-previews", admin(s.startPreview))
	mux.HandleFunc("GET /api/library-previews/{id}", admin(s.getPreview))
	mux.HandleFunc("GET /api/folders", admin(s.listFolders))
	mux.HandleFunc("GET /api/backups", admin(s.listBackups))
	mux.HandleFunc("POST /api/backups", admin(s.createBackup))
	mux.HandleFunc("POST /api/backups/{name}/restore", admin(s.restoreBackup))
	mux.HandleFunc("DELETE /api/backups/{name}", admin(s.deleteBackup))
	mux.HandleFunc("PATCH /api/items/{id}", admin(s.editItem))
	mux.HandleFunc("POST /api/items/{id}/refresh", admin(s.refreshItem))
	mux.HandleFunc("GET /api/items/{id}/artwork", admin(s.listArtwork))
	mux.HandleFunc("PUT /api/items/{id}/artwork", admin(s.chooseArtwork))
	mux.HandleFunc("POST /api/items/{id}/artwork/{type}", admin(s.uploadArtwork))
	mux.HandleFunc("GET /api/items/{id}/match", admin(s.matchInfo))
	mux.HandleFunc("GET /api/items/{id}/match/search", admin(s.searchMatches))
	mux.HandleFunc("PUT /api/items/{id}/match", admin(s.setMatch))
	mux.HandleFunc("POST /api/series/{id}/orderings", admin(s.createOrdering))
	mux.HandleFunc("DELETE /api/series/{id}/orderings/{oid}", admin(s.deleteOrdering))
	mux.HandleFunc("PUT /api/series/{id}/orderings/{oid}/entries/{eid}", admin(s.setEntry))
	mux.HandleFunc("DELETE /api/series/{id}/orderings/{oid}/entries/{eid}", admin(s.clearEntry))
	mux.HandleFunc("POST /api/series/{id}/refresh", admin(s.refreshSeries))
	mux.HandleFunc("GET /api/series/{id}/season-art", admin(s.seasonArtList))
	mux.HandleFunc("PATCH /api/series/{id}/seasons/{n}", admin(s.editSeason))
	mux.HandleFunc("GET /api/series/{id}/seasons/{n}/artwork", admin(s.seasonArtwork))
	mux.HandleFunc("PUT /api/series/{id}/seasons/{n}/artwork", admin(s.chooseSeasonArtwork))
	mux.HandleFunc("POST /api/series/{id}/seasons/{n}/artwork", admin(s.uploadSeasonArtwork))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no such endpoint")
	})
	mux.Handle("/", webui.Handler())
	return s.logRequests(mux)
}

// itemDTO adds links and files to a stored item.
type itemDTO struct {
	store.Item
	// For episodes, where the caller asks for it: the series' title, and the episode's place in the
	// profile's episode order.
	SeriesTitle   string `json:"series_title,omitempty"`
	SeasonNumber  *int   `json:"season_number,omitempty"`
	EpisodeNumber *int   `json:"episode_number,omitempty"`
	// The current profile's watch state: for movies and episodes, and for series the number of
	// episodes on disk not watched yet.
	UserData       *store.UserData `json:"user_data,omitempty"`
	UnwatchedCount *int            `json:"unwatched_count,omitempty"`
	// On a movie or show's own page: its genres, cast and crew.
	Genres  []string    `json:"genres,omitempty"`
	Labels  []string    `json:"labels,omitempty"`
	Credits []creditDTO `json:"credits,omitempty"`
	Extras  []extraDTO  `json:"extras,omitempty"`
	// On a movie or show's own page: the collections it's in.
	Collections []collectionRef   `json:"collections,omitempty"`
	Images      map[string]string `json:"images,omitempty"`
	Stream      string            `json:"stream_url,omitempty"`
	// The title's theme tune, when there's one beside it.
	Theme string            `json:"theme_url,omitempty"`
	Files []store.MediaFile `json:"files,omitempty"`
}

// imagePaths returns an item's provider (or uploaded) image path and the local artwork file to use
// instead, for "primary", "backdrop" or "logo". Artwork the user chose (the field is locked) wins over local
// files, which win over the provider's automatic choice.
func imagePaths(it *store.Item, typ string) (path, local string) {
	field := "poster"
	path, local = it.PosterPath, it.LocalPoster
	switch typ {
	case "backdrop":
		field = "backdrop"
		path, local = it.BackdropPath, it.LocalBackdrop
	case "logo":
		field = "logo"
		path, local = it.LogoPath, it.LocalLogo
	}
	if path != "" && slices.Contains(it.LockedFields, field) {
		local = ""
	}
	return path, local
}

func toDTO(it store.Item) itemDTO {
	d := itemDTO{Item: it, Images: map[string]string{}}
	base := "/api/items/" + it.ID
	// The version changes with the image, so each link can be cached by browsers forever.
	for _, typ := range []string{"primary", "backdrop", "logo"} {
		path, local := imagePaths(&it, typ)
		switch {
		case local != "":
			d.Images[typ] = base + "/images/" + typ + "?local=" + artwork.Key(local)
		case path != "":
			d.Images[typ] = base + "/images/" + typ + "?v=" + artwork.Key(artwork.Source(it.Kind, typ, path))
		}
	}
	if it.Kind == "movie" || it.Kind == "episode" {
		d.Stream = base + "/stream"
	}
	if it.LocalTheme != "" {
		d.Theme = base + "/theme?v=" + artwork.Key(it.LocalTheme)
	}
	return d
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"name": "Arc Media Server", "version": s.version, "status": "ok"})
}

func (s *Server) listLibraries(w http.ResponseWriter, r *http.Request) {
	libs, err := s.store.ListLibraries(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	counts, err := s.store.LibraryCounts(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	type libDTO struct {
		store.Library
		// How many movies or shows it lists.
		Count int            `json:"count"`
		Scan  scanner.Status `json:"scan"`
	}
	out := make([]libDTO, 0, len(libs))
	acc := access(r)
	for _, l := range libs {
		if acc.LibraryAllowed(l.ID) {
			out = append(out, libDTO{Library: l, Count: counts[l.ID], Scan: s.scanner.Status(l.ID)})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) libraryFromPath(w http.ResponseWriter, r *http.Request) (*store.Library, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid library id")
		return nil, false
	}
	lib, err := s.store.GetLibrary(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return nil, false
	}
	if lib == nil || !access(r).LibraryAllowed(lib.ID) {
		writeError(w, http.StatusNotFound, "library not found")
		return nil, false
	}
	return lib, true
}

func (s *Server) scanStatus(w http.ResponseWriter, r *http.Request) {
	if lib, ok := s.libraryFromPath(w, r); ok {
		writeJSON(w, http.StatusOK, s.scanner.Status(lib.ID))
	}
}

func (s *Server) startScan(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	if !s.scanner.Start(lib.ID) {
		writeError(w, http.StatusConflict, "a scan is already running for this library")
		return
	}
	writeJSON(w, http.StatusAccepted, s.scanner.Status(lib.ID))
}

func (s *Server) listItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := store.ItemQuery{
		Kind:   q.Get("kind"),
		Search: strings.TrimSpace(q.Get("q")),
		Sort:   q.Get("sort"),
	}
	query.LibraryID, _ = strconv.ParseInt(q.Get("library_id"), 10, 64)
	query.Limit, _ = strconv.Atoi(q.Get("limit"))
	query.Offset, _ = strconv.Atoi(q.Get("offset"))
	query.Access = access(r)
	query.Genre = q.Get("genre")
	query.Language = q.Get("language")
	query.Label = q.Get("label")
	query.Decade, _ = strconv.Atoi(q.Get("decade"))
	query.Resolution = q.Get("resolution")
	query.UserID = currentUserID(r)
	query.Unwatched = q.Get("unwatched") == "1" || q.Get("unwatched") == "true"
	query.AddedWithinDays, _ = strconv.Atoi(q.Get("added_within"))
	query.MinRuntime, _ = strconv.Atoi(q.Get("min_runtime"))
	query.MaxRuntime, _ = strconv.Atoi(q.Get("max_runtime"))
	query.MinRating, _ = strconv.ParseFloat(q.Get("min_rating"), 64)
	if v := q.Get("about"); v != "" {
		query.About = strings.Split(v, ",")
	}
	if query.Sort != "" && !store.ItemSorts[query.Sort] {
		writeError(w, http.StatusBadRequest, "unknown sort")
		return
	}

	items, total, err := s.store.ListItems(r.Context(), query)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := toDTOs(items)
	if err := s.fillUserData(r, ptrs(out)); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "items": out})
}

func (s *Server) itemFromPath(w http.ResponseWriter, r *http.Request) (*store.Item, bool) {
	it, err := s.store.GetItem(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return nil, false
	}
	if it == nil {
		writeError(w, http.StatusNotFound, "item not found")
		return nil, false
	}
	// Titles above a profile's age limit don't exist as far as that profile is concerned.
	if ok, err := s.store.CanSee(r.Context(), it, access(r)); err != nil {
		s.fail(w, err)
		return nil, false
	} else if !ok {
		writeError(w, http.StatusNotFound, "item not found")
		return nil, false
	}
	return it, true
}

// maxAge is the current profile's age limit (0 = none).
// access is what the current profile may see: its age limit and libraries. The admin sees all.
func access(r *http.Request) store.Access {
	u := currentUser(r)
	if u == nil || u.IsAdmin {
		return store.Access{}
	}
	return store.Access{MaxAge: u.MaxAge, Libraries: u.LibraryIDs, ExcludeLabels: u.ExcludeLabels}
}

func (s *Server) getItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	files, err := s.store.ItemFiles(r.Context(), it.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	d := toDTO(*it)
	d.Files = files
	if err := s.addDetails(r, &d); err != nil {
		s.fail(w, err)
		return
	}
	if err := s.fillUserData(r, []*itemDTO{&d}); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

var videoMIME = map[string]string{
	".mp4": "video/mp4", ".m4v": "video/x-m4v", ".mkv": "video/x-matroska", ".webm": "video/webm",
	".avi": "video/x-msvideo", ".mov": "video/quicktime", ".ts": "video/mp2t", ".m2ts": "video/mp2t",
	".wmv": "video/x-ms-wmv", ".mpg": "video/mpeg", ".mpeg": "video/mpeg", ".flv": "video/x-flv",
}

// stream serves the original file with HTTP range support (direct play). ?file=<id> selects a
// specific part or version; otherwise the first available file is used.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	files, err := s.store.ItemFiles(r.Context(), it.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	wantID, _ := strconv.ParseInt(r.URL.Query().Get("file"), 10, 64)
	var chosen *store.MediaFile
	for i := range files {
		f := &files[i]
		if f.Missing || (wantID != 0 && f.ID != wantID) {
			continue
		}
		chosen = f
		break
	}
	if chosen == nil {
		writeError(w, http.StatusNotFound, "no playable file for this item")
		return
	}
	stop, done, refused := s.playing.openStream(r, it.ID, "")
	if refused != "" {
		writeError(w, http.StatusForbidden, refused)
		return
	}
	defer done()
	s.serveFile(w, r, chosen.Path, stop)
}

// serveFile sends a media file as it is, with range requests for seeking. Closing stop cuts the
// response off part-way.
func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, path string, stop chan struct{}) {
	fh, err := os.Open(path)
	if err != nil {
		s.log.Warn("cannot open media file", "path", path, "err", err)
		writeError(w, http.StatusNotFound, "media file unavailable")
		return
	}
	defer fh.Close()
	info, err := fh.Stat()
	if err != nil {
		s.fail(w, err)
		return
	}
	if mime := videoMIME[strings.ToLower(filepath.Ext(path))]; mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), stoppableFile{fh, stop})
}

// image serves an item's artwork from the local cache, downloading it on first use. If it can't
// be downloaded (e.g. the provider is unreachable) the browser is sent to the provider instead.
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	typ := r.PathValue("type")
	if typ != "primary" && typ != "backdrop" && typ != "logo" {
		writeError(w, http.StatusBadRequest, "unknown image type")
		return
	}
	imagePath, local := imagePaths(it, typ)
	// Uploaded artwork lives in the data folder.
	if name, ok := strings.CutPrefix(imagePath, metadata.UploadPrefix); ok {
		local = filepath.Join(s.uploadsDir, filepath.Base(name))
	}
	// Artwork next to the media is read straight from disk; the browser checks back each time, so
	// a replaced poster.jpg shows up.
	if local != "" {
		if f, err := os.Open(local); err == nil {
			defer f.Close()
			if info, err := f.Stat(); err == nil {
				w.Header().Set("Cache-Control", "no-cache")
				http.ServeContent(w, r, filepath.Base(local), info.ModTime(), f)
				return
			}
		}
	}
	if imagePath == "" {
		writeError(w, http.StatusNotFound, "no image")
		return
	}
	s.serveArtwork(w, r, artwork.Source(it.Kind, r.PathValue("type"), imagePath))
}

// serveArtwork serves an image from the cache (downloading it if needed), cacheable forever when
// the link carries its version; if it can't be had, the browser is sent to the provider.
func (s *Server) serveArtwork(w http.ResponseWriter, r *http.Request, src string) {
	f, err := s.artwork.Open(r.Context(), src)
	if err != nil {
		s.log.Debug("artwork unavailable locally", "url", src, "err", err)
		http.Redirect(w, r, src, http.StatusFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		s.fail(w, err)
		return
	}
	key := artwork.Key(src)
	if r.URL.Query().Get("v") == key {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("ETag", `"`+key+`"`)
	http.ServeContent(w, r, artwork.ContentName(src), info.ModTime(), f)
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	s.log.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"took", time.Since(start).Round(time.Millisecond))
	})
}

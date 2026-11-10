package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"ams/internal/importer"
	"ams/internal/store"
)

// Importing from Plex or Jellyfin is a short conversation: sign in (Plex: approve Arc on plex.tv),
// choose a server, match its people to Arc profiles, then import in the background. Each import
// is a session retained while running and for an hour after completion.

type importSession struct {
	mu        sync.Mutex
	preparing bool
	id        string
	source    string // plex or jellyfin
	stage     string // signing_in, choose_server, choose_accounts, running, done, failed
	login     *importer.PlexLogin
	plexToken string
	servers   []importer.PlexServer
	server    string
	src       importer.Source
	accounts  []importer.Account
	results   []importer.AccountResult
	lists     *importer.ListsResult
	err       string
	created   time.Time
}

type importSessions struct {
	mu   sync.Mutex
	byID map[string]*importSession
}

func (s *importSessions) add(sess *importSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byID == nil {
		s.byID = map[string]*importSession{}
	}
	for id, old := range s.byID {
		old.mu.Lock()
		expired := old.expired(time.Now())
		old.mu.Unlock()
		if expired {
			delete(s.byID, id)
		}
	}
	s.byID[sess.id] = sess
}

func (s *importSessions) get(id string) *importSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.byID[id]
	if sess != nil {
		sess.mu.Lock()
		expired := sess.expired(time.Now())
		sess.mu.Unlock()
		if expired {
			delete(s.byID, id)
			return nil
		}
	}
	return sess
}

// Call with sess.mu held. Running/preparing jobs never expire out from under polling.
func (sess *importSession) expired(now time.Time) bool {
	return !sess.preparing && sess.stage != "running" && now.Sub(sess.created) > time.Hour
}

var importClient = &http.Client{Timeout: 2 * time.Minute}

// clientID is this Arc server's stable identity with Plex.
func (s *Server) plexClientID(ctx context.Context) (string, error) {
	id, err := s.store.Setting(ctx, "plex_client_id")
	if err != nil || id != "" {
		return id, err
	}
	id = "arc-" + store.NewID()
	return id, s.store.SetSetting(ctx, "plex_client_id", id)
}

// startImport: POST /api/imports {"source": "plex"} or {"source": "jellyfin", "url", "username",
// "password"} (admin).
func (s *Server) startImport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source   string `json:"source"`
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	ctx := r.Context()
	clientID, err := s.plexClientID(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	sess := &importSession{id: store.NewID(), source: body.Source, created: time.Now()}
	switch body.Source {
	case "plex":
		login, err := importer.StartPlexLogin(ctx, importClient, clientID)
		if err != nil {
			writeError(w, http.StatusBadGateway, "couldn't reach plex.tv: "+err.Error())
			return
		}
		sess.login, sess.stage = login, "signing_in"
	case "jellyfin":
		if strings.TrimSpace(body.URL) == "" || body.Username == "" {
			writeError(w, http.StatusBadRequest, "enter the Jellyfin server's address and an administrator's user name")
			return
		}
		j, err := importer.ConnectJellyfin(ctx, importClient, body.URL, clientID, body.Username, body.Password)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		sess.src = j
		if sess.accounts, err = j.Accounts(ctx); err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		sess.stage = "choose_accounts"
	default:
		writeError(w, http.StatusBadRequest, `source is "plex" or "jellyfin"`)
		return
	}
	s.imports.add(sess)
	s.writeImport(w, r, sess)
}

func (s *Server) importFromPath(w http.ResponseWriter, r *http.Request) *importSession {
	sess := s.imports.get(r.PathValue("id"))
	if sess == nil {
		writeError(w, http.StatusNotFound, "that import has expired; start again")
	}
	return sess
}

// getImport: GET /api/imports/{id} (admin). While signing in to Plex, each look checks whether
// the person has approved Arc yet.
func (s *Server) getImport(w http.ResponseWriter, r *http.Request) {
	sess := s.importFromPath(w, r)
	if sess == nil {
		return
	}
	sess.mu.Lock()
	signing := sess.stage == "signing_in"
	login := sess.login
	sess.mu.Unlock()
	if signing {
		ctx := r.Context()
		clientID, err := s.plexClientID(ctx)
		if err != nil {
			s.fail(w, err)
			return
		}
		token, err := importer.CheckPlexLogin(ctx, importClient, clientID, login)
		if err == nil && token != "" {
			servers, serr := importer.PlexServers(ctx, importClient, clientID, token)
			sess.mu.Lock()
			if sess.stage != "signing_in" {
				sess.mu.Unlock()
				s.writeImport(w, r, sess)
				return
			}
			if serr != nil {
				sess.stage, sess.err = "failed", "couldn't list your Plex servers: "+serr.Error()
			} else if len(servers) == 0 {
				sess.stage, sess.err = "failed", "this Plex account has no servers"
			} else {
				sess.plexToken, sess.servers, sess.stage = token, servers, "choose_server"
			}
			sess.mu.Unlock()
		}
	}
	s.writeImport(w, r, sess)
}

// chooseImportServer: POST /api/imports/{id}/server {"server_id"} (admin) connects to a Plex server.
func (s *Server) chooseImportServer(w http.ResponseWriter, r *http.Request) {
	sess := s.importFromPath(w, r)
	if sess == nil {
		return
	}
	var body struct {
		ServerID string `json:"server_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	sess.mu.Lock()
	if sess.preparing || (sess.stage != "choose_server" && sess.stage != "choose_accounts" && sess.stage != "done") {
		sess.mu.Unlock()
		writeError(w, http.StatusConflict, "this import cannot change servers now")
		return
	}
	sess.preparing = true
	sess.mu.Unlock()
	defer func() { sess.mu.Lock(); sess.preparing = false; sess.mu.Unlock() }()
	sess.mu.Lock()
	var chosen *importer.PlexServer
	for i := range sess.servers {
		if sess.servers[i].ID == body.ServerID {
			chosen = &sess.servers[i]
		}
	}
	sess.mu.Unlock()
	if chosen == nil {
		writeError(w, http.StatusBadRequest, "choose one of the listed servers")
		return
	}
	ctx := r.Context()
	clientID, err := s.plexClientID(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	p, err := importer.ConnectPlex(ctx, importClient, clientID, *chosen)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// Its watchlist is on plex.tv, read with the account the person signed in with.
	p.SetAccountToken(sess.plexToken)
	accounts, err := p.Accounts(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "couldn't list the server's accounts: "+err.Error())
		return
	}
	sess.mu.Lock()
	sess.src, sess.accounts, sess.server, sess.stage = p, accounts, chosen.Name, "choose_accounts"
	sess.mu.Unlock()
	s.writeImport(w, r, sess)
}

// runImport: POST /api/imports/{id}/run {"profiles": {"<account id>": "<profile id>" | "new" | ""}}
// (admin) imports each chosen account into its profile ("new" makes a profile named after it).
func (s *Server) runImport(w http.ResponseWriter, r *http.Request) {
	releaseCatalog, ok := s.store.BeginCatalogWork()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "catalog maintenance in progress; retry shortly")
		return
	}
	transferred := false
	defer func() {
		if !transferred {
			releaseCatalog()
		}
	}()
	sess := s.importFromPath(w, r)
	if sess == nil {
		return
	}
	var body struct {
		Profiles map[string]string `json:"profiles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	sess.mu.Lock()
	if sess.preparing || (sess.stage != "choose_accounts" && sess.stage != "done") {
		sess.mu.Unlock()
		writeError(w, http.StatusConflict, "this import isn't ready to run")
		return
	}
	sess.preparing = true
	accounts, src := append([]importer.Account(nil), sess.accounts...), sess.src
	sess.mu.Unlock()
	defer func() { sess.mu.Lock(); sess.preparing = false; sess.mu.Unlock() }()

	root := s.workContext
	if root == nil {
		root = context.Background()
	}
	if root.Err() != nil {
		writeError(w, http.StatusServiceUnavailable, "server is stopping; try again after restart")
		return
	}
	// Register before preflight so server shutdown also joins in-flight admission.
	if s.workers != nil {
		s.workers.Add(1)
	}
	registered := true
	defer func() {
		if registered && s.workers != nil {
			s.workers.Done()
		}
	}()
	ctx, cancelPreflight := context.WithCancel(r.Context())
	stopPreflight := context.AfterFunc(root, cancelPreflight)
	defer func() { stopPreflight(); cancelPreflight() }()
	known := map[string]bool{}
	var targets []store.ImportProfileTarget
	for _, a := range accounts {
		known[a.ID] = true
		targets = append(targets, store.ImportProfileTarget{AccountID: a.ID, Name: a.Name, Target: body.Profiles[a.ID]})
	}
	for id := range body.Profiles {
		if !known[id] {
			writeError(w, http.StatusBadRequest, "choose only listed import accounts")
			return
		}
	}
	if src == nil {
		writeError(w, http.StatusConflict, "connect to the import server again")
		return
	}
	releaseCatalog, ok := s.store.BeginCatalogWork()
	if !ok {
		w.Header().Set("Retry-After", "3")
		writeError(w, http.StatusServiceUnavailable, "catalog maintenance is in progress; try again shortly")
		return
	}
	releaseOnReturn := true
	defer func() {
		if releaseOnReturn {
			releaseCatalog()
		}
	}()
	mapping, names, err := s.store.PrepareImportProfiles(ctx, targets, sess.sourceName())
	if err != nil {
		if ctx.Err() != nil {
			if r.Context().Err() == nil {
				writeError(w, http.StatusServiceUnavailable, "server is stopping; try again after restart")
			}
			return
		}
		switch {
		case errors.Is(err, store.ErrInvalidImportProfiles):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, store.ErrNameTaken):
			writeError(w, http.StatusConflict, "an imported profile name is already taken; choose an existing profile")
		default:
			s.fail(w, err)
		}
		return
	}
	sess.mu.Lock()
	sess.stage, sess.err, sess.results, sess.lists = "running", "", nil, nil
	sess.mu.Unlock()
	registered = false
	releaseOnReturn = false
	go func() {
		if s.workers != nil {
			defer s.workers.Done()
		}
		defer releaseCatalog()
		started := time.Now()
		s.log.Info("import started", "source", sess.source, "accounts", len(mapping))
		bg, cancel := context.WithTimeout(root, 3*time.Hour)
		defer cancel()
		task := s.activity.Start("import", "Importing from "+sess.sourceName())
		results, lists, err := importer.Run(bg, s.store, src, accounts, mapping, names, task)
		taskErr := err
		if err != nil {
			taskErr = errors.New("The import could not finish. Check the source connection and try again.")
		}
		if bg.Err() != nil {
			taskErr = bg.Err()
		}
		task.Finish(taskErr)
		sess.mu.Lock()
		sess.results, sess.lists = results, &lists
		sess.created = time.Now() // retain the finished result for a full hour
		if err != nil {
			sess.stage, sess.err = "failed", "The import could not finish. Check the source connection and try again."
			if bg.Err() != nil {
				sess.err = "The import was interrupted; run it again to continue merging watch state."
				s.log.Info("import interrupted", "source", sess.source, "accounts", len(results), "took", time.Since(started))
			} else {
				operation, reason := "write_watch_state", "database operation failed"
				var sourceFailure *importer.SourceError
				if errors.As(err, &sourceFailure) {
					operation, reason = "read_account", "source request failed"
				}
				s.log.Warn("import failed", "source", sess.source, "operation", operation, "reason", reason, "accounts", len(results), "took", time.Since(started))
			}
		} else {
			sess.stage = "done"
			s.log.Info("import finished", "source", sess.source, "accounts", len(results), "took", time.Since(started))
		}
		sess.mu.Unlock()
	}()
	s.writeImport(w, r, sess)
}

func (sess *importSession) sourceName() string {
	if sess.source == "plex" {
		return "Plex"
	}
	return "Jellyfin"
}

func (s *Server) writeImport(w http.ResponseWriter, r *http.Request, sess *importSession) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	type server struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Owned bool   `json:"owned"`
	}
	type account struct {
		importer.Account
		// The profile it most likely is: one with the same name, or the admin for Plex's owner.
		Suggested string `json:"suggested_profile,omitempty"`
	}
	out := struct {
		ID       string                   `json:"id"`
		Source   string                   `json:"source"`
		Stage    string                   `json:"stage"`
		AuthURL  string                   `json:"auth_url,omitempty"`
		Servers  []server                 `json:"servers,omitempty"`
		Server   string                   `json:"server,omitempty"`
		Accounts []account                `json:"accounts,omitempty"`
		Results  []importer.AccountResult `json:"results,omitempty"`
		// The server's collections, brought into the libraries their titles are in.
		Collections *importer.ListsResult `json:"collections,omitempty"`
		Error       string                `json:"error,omitempty"`
	}{ID: sess.id, Source: sess.source, Stage: sess.stage, Server: sess.server, Results: sess.results, Collections: sess.lists, Error: sess.err}
	if sess.stage == "signing_in" && sess.login != nil {
		out.AuthURL = sess.login.AuthURL
	}
	for _, sv := range sess.servers {
		out.Servers = append(out.Servers, server{ID: sv.ID, Name: sv.Name, Owned: sv.Owned})
	}
	for _, a := range sess.accounts {
		ac := account{Account: a}
		for _, u := range users {
			if strings.EqualFold(u.Name, a.Name) || (a.Owner && u.IsAdmin && ac.Suggested == "") {
				ac.Suggested = u.ID
			}
		}
		out.Accounts = append(out.Accounts, ac)
	}
	writeJSON(w, http.StatusOK, out)
}

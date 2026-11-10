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
// is a session kept in memory for an hour.

type importSession struct {
	mu        sync.Mutex
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
		if time.Since(old.created) > time.Hour {
			delete(s.byID, id)
		}
	}
	s.byID[sess.id] = sess
}

func (s *importSessions) get(id string) *importSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byID[id]
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
	if sess.stage != "choose_accounts" && sess.stage != "done" {
		sess.mu.Unlock()
		writeError(w, http.StatusConflict, "this import isn't ready to run")
		return
	}
	accounts, src := sess.accounts, sess.src
	sess.mu.Unlock()

	ctx := r.Context()
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	names := map[string]string{}
	for _, u := range users {
		names[u.ID] = u.Name
	}
	mapping := map[string]string{}
	for _, a := range accounts {
		target := body.Profiles[a.ID]
		switch {
		case target == "":
			continue
		case target == "new":
			u, err := s.store.CreateUser(ctx, a.Name, false, "slate", "", "")
			if errors.Is(err, store.ErrNameTaken) {
				u, err = s.store.CreateUser(ctx, a.Name+" ("+sess.sourceName()+")", false, "slate", "", "")
			}
			if err != nil {
				s.fail(w, err)
				return
			}
			target, names[u.ID] = u.ID, u.Name
		case names[target] == "":
			writeError(w, http.StatusBadRequest, "no such profile for "+a.Name)
			return
		}
		mapping[a.ID] = target
	}
	if len(mapping) == 0 {
		writeError(w, http.StatusBadRequest, "choose a profile for at least one person")
		return
	}
	sess.mu.Lock()
	sess.stage, sess.err, sess.results, sess.lists = "running", "", nil, nil
	sess.mu.Unlock()
	if !s.launchWork(func(root context.Context) {
		bg, cancel := context.WithTimeout(root, 3*time.Hour)
		defer cancel()
		task := s.activity.Start("import", "Importing from "+sess.sourceName())
		results, lists, err := importer.Run(bg, s.store, src, accounts, mapping, names, task)
		task.Finish(err)
		sess.mu.Lock()
		sess.results, sess.lists = results, &lists
		if err != nil {
			sess.stage, sess.err = "failed", err.Error()
			s.log.Warn("import failed", "source", sess.source, "err", err)
		} else {
			sess.stage = "done"
			s.log.Info("import finished", "source", sess.source, "accounts", len(results))
		}
		sess.mu.Unlock()
	}) {
		sess.mu.Lock()
		sess.stage, sess.err = "failed", "server is stopping"
		sess.mu.Unlock()
	}
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

package api

import (
	"ams/internal/events"
	"ams/internal/store"
	"ams/internal/watchtogether"
	"net/http"
)

// Watch Together by invitation: the host picks people, the room appears in their
// room lists (Home's Watch Together row), and they join from there. Codes still work alongside.

type watchPersonDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	// A friend (outside the household) rather than a household profile.
	Friend bool `json:"friend"`
	// Whether they may play the title asked about (item_id); true when none was given.
	CanWatch bool `json:"can_watch"`
}

type watchListedDTO struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	PositionMS  int64  `json:"position_ms"`
	CreatedAtMS int64  `json:"created_at_ms"`
	HostName    string `json:"host_name"`
	// This profile made the room (it's host when it joins).
	Hosting bool `json:"hosting"`
	// This device and profile are in the room now.
	Joined       bool         `json:"joined"`
	Item         watchItemDTO `json:"item"`
	Participants []string     `json:"participants"`
	Invited      []string     `json:"invited"`
}

type watchCreatedDTO struct {
	Room              watchListedDTO `json:"room"`
	InviteCode        string         `json:"invite_code"`
	InviteExpiresAtMS int64          `json:"invite_expires_at_ms"`
}

// wireWatchNotify sends a personal "watch_together" hint when a profile's room list changes.
func (s *Server) wireWatchNotify() {
	if s.watchTogether == nil || s.events == nil {
		return
	}
	hub := s.events
	s.watchTogether.Notify = func(ids []string) {
		for _, id := range ids {
			hub.Publish(events.Event{Topic: "watch_together", UserID: id})
		}
	}
}

// watchPeople is who a profile may invite: household profiles see everyone else (household and
// friends); friends see the household's admins, never its other profiles.
func (s *Server) watchPeople(r *http.Request, u *store.User) ([]store.User, error) {
	all, err := s.store.ListUsers(r.Context())
	if err != nil {
		return nil, err
	}
	out := []store.User{}
	for _, p := range all {
		if p.ID == u.ID || (u.IsFriend && !p.IsAdmin) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// peopleWatchTogether: GET /api/watch-together/people?item_id= lists who this profile may invite,
// and whether each may play the title. Names and colours only.
func (s *Server) peopleWatchTogether(w http.ResponseWriter, r *http.Request) {
	_, u, err := s.watchActor(r)
	if err != nil {
		s.watchFailure(w, r, watchtogether.Snapshot{}, err)
		return
	}
	people, err := s.watchPeople(r, u)
	if err != nil {
		s.fail(w, err)
		return
	}
	item := r.URL.Query().Get("item_id")
	out := []watchPersonDTO{}
	for i := range people {
		p := &people[i]
		can := true
		if item != "" {
			it, err := s.watchItem(r.Context(), item, p)
			if err != nil {
				s.fail(w, err)
				return
			}
			can = it != nil
		}
		out = append(out, watchPersonDTO{ID: p.ID, Name: p.Name, Color: p.Color, Friend: p.IsFriend, CanWatch: can})
	}
	writeJSON(w, 200, map[string]any{"people": out})
}

func (s *Server) watchListed(r *http.Request, room watchtogether.Snapshot, a watchtogether.Actor, it *store.Item) watchListedDTO {
	out := watchListedDTO{ID: room.ID, State: room.State, PositionMS: room.PositionMS, CreatedAtMS: room.CreatedMS, HostName: room.HostName, Hosting: room.HostUserID == a.UserID,
		Item: watchItemDTO{ID: it.ID, Title: it.Title, Kind: it.Kind}, Participants: []string{}, Invited: []string{}}
	for _, p := range room.Members {
		out.Participants = append(out.Participants, p.Actor.Name)
		if p.Actor.Key() == a.Key() {
			out.Joined = true
		}
	}
	for _, p := range room.Invited {
		out.Invited = append(out.Invited, p.Name)
	}
	return out
}

// listWatchTogether: GET /api/watch-together/rooms lists the rooms this profile made, was invited
// to or is in, newest first, for Home's Watch Together row. Rooms whose title it may not play are
// left out. Rooms are private: nobody else's appear.
func (s *Server) listWatchTogether(w http.ResponseWriter, r *http.Request) {
	a, u, err := s.watchActor(r)
	if err != nil {
		s.watchFailure(w, r, watchtogether.Snapshot{}, err)
		return
	}
	out := []watchListedDTO{}
	for _, room := range s.watchTogether.Rooms(u.ID) {
		it, err := s.watchItem(r.Context(), room.ItemID, u)
		if err != nil {
			s.fail(w, err)
			return
		}
		if it == nil {
			continue
		}
		out = append(out, s.watchListed(r, room, a, it))
	}
	writeJSON(w, 200, map[string]any{"rooms": out})
}

// invitees checks the chosen profiles: each must be someone this profile may invite and may
// play the title.
func (s *Server) invitees(r *http.Request, u *store.User, ids []string, item string) ([]watchtogether.Invitee, bool, error) {
	people, err := s.watchPeople(r, u)
	if err != nil {
		return nil, false, err
	}
	byID := map[string]*store.User{}
	for i := range people {
		byID[people[i].ID] = &people[i]
	}
	out := []watchtogether.Invitee{}
	for _, id := range ids {
		p := byID[id]
		if p == nil {
			return nil, false, nil
		}
		it, err := s.watchItem(r.Context(), item, p)
		if err != nil {
			return nil, false, err
		}
		if it == nil {
			return nil, false, nil
		}
		out = append(out, watchtogether.Invitee{UserID: p.ID, Name: p.Name})
	}
	return out, true, nil
}

// createInvitedWatchTogether: POST /api/watch-together/rooms {item_id,start_ms,profile_ids} makes
// a room for the chosen people: nobody is in it until they join from their room
// lists (the host too). It waits an hour for them. Each person must be someone this profile may
// invite (GET .../people) and may play the title. The code is returned as for any room.
func (s *Server) createInvitedWatchTogether(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ItemID     string   `json:"item_id"`
		StartMS    int64    `json:"start_ms" api:"optional"`
		ProfileIDs []string `json:"profile_ids"`
	}
	if decodeWatch(w, r, &body) != nil || body.StartMS < 0 || body.StartMS > watchtogether.MaxPositionMS || len(body.ProfileIDs) == 0 || len(body.ProfileIDs) >= watchtogether.MaxParticipants {
		writeError(w, 400, "send item_id, profile_ids (1 to 9) and optional nonnegative start_ms")
		return
	}
	a, u, err := s.watchActor(r)
	if err != nil {
		s.watchFailure(w, r, watchtogether.Snapshot{}, err)
		return
	}
	if !s.watchAdmission(w, r, a) {
		return
	}
	it, err := s.watchItem(r.Context(), body.ItemID, u)
	if err != nil {
		s.fail(w, err)
		return
	}
	if it == nil {
		writeError(w, 404, "playable movie or episode not found")
		return
	}
	people, ok, err := s.invitees(r, u, body.ProfileIDs, it.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !ok {
		writeError(w, 400, "everyone invited must be someone you can invite who may play this title")
		return
	}
	room, code, expiry, err := s.watchTogether.CreateInvited(a, it.ID, body.StartMS, people)
	if err != nil {
		s.watchFailure(w, r, room, err)
		return
	}
	writeJSON(w, 201, watchCreatedDTO{Room: s.watchListed(r, room, a, it), InviteCode: code, InviteExpiresAtMS: expiry.UnixMilli()})
}

// acceptWatchTogether: POST /api/watch-together/rooms/{id}/join joins a room from this profile's
// list (no code), if it may play the room's title. Returns the personalized room.
func (s *Server) acceptWatchTogether(w http.ResponseWriter, r *http.Request) {
	a, u, err := s.watchActor(r)
	if err != nil {
		s.watchFailure(w, r, watchtogether.Snapshot{}, err)
		return
	}
	if !s.watchAdmission(w, r, a) {
		return
	}
	item, err := s.watchTogether.RoomItem(r.PathValue("id"), u.ID)
	if err != nil {
		s.watchFailure(w, r, watchtogether.Snapshot{}, err)
		return
	}
	it, err := s.watchItem(r.Context(), item, u)
	if err != nil {
		s.fail(w, err)
		return
	}
	if it == nil {
		writeError(w, 404, "room or playable invitation not found")
		return
	}
	room, err := s.watchTogether.Accept(a, r.PathValue("id"), it.ID)
	if err != nil {
		s.watchFailure(w, r, room, err)
		return
	}
	d, err := s.watchView(r, room)
	if err != nil {
		s.watchFailure(w, r, room, err)
		return
	}
	writeJSON(w, 200, d)
}

// dismissWatchTogether: POST /api/watch-together/rooms/{id}/dismiss takes a room off this
// profile's list. Someone invited declines; the profile that made it cancels it for everyone.
func (s *Server) dismissWatchTogether(w http.ResponseWriter, r *http.Request) {
	_, u, err := s.watchActor(r)
	if err != nil {
		s.watchFailure(w, r, watchtogether.Snapshot{}, err)
		return
	}
	if err := s.watchTogether.Dismiss(r.PathValue("id"), u.ID); err != nil {
		s.watchFailure(w, r, watchtogether.Snapshot{}, err)
		return
	}
	w.WriteHeader(204)
}

// inviteWatchTogether: POST /api/watch-together/{id}/people {profile_ids} invites more people to
// a room (host only, members only). Returns the personalized room with them in `invited`.
func (s *Server) inviteWatchTogether(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProfileIDs []string `json:"profile_ids"`
	}
	if decodeWatch(w, r, &body) != nil || len(body.ProfileIDs) == 0 || len(body.ProfileIDs) >= watchtogether.MaxParticipants {
		writeError(w, 400, "send profile_ids (1 to 9)")
		return
	}
	room, a, ok := s.watchRoom(w, r)
	if !ok {
		return
	}
	u, err := s.store.GetUser(r.Context(), a.UserID)
	if err != nil || u == nil {
		s.watchFailure(w, r, watchtogether.Snapshot{}, errWatchAuth)
		return
	}
	people, valid, err := s.invitees(r, u, body.ProfileIDs, room.ItemID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !valid {
		writeError(w, 400, "everyone invited must be someone you can invite who may play this title")
		return
	}
	next, err := s.watchTogether.Invite(room.ID, a.Key(), people)
	if err != nil {
		s.watchFailure(w, r, next, err)
		return
	}
	d, err := s.watchView(r, next)
	if err != nil {
		s.watchFailure(w, r, next, err)
		return
	}
	writeJSON(w, 200, d)
}

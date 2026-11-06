package api

import (
	"ams/internal/store"
	"ams/internal/watchtogether"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

var errWatchAuth = errors.New("watch together session changed")

type watchMemberDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Ready     bool   `json:"ready"`
	State     string `json:"state"`
}
type watchItemDTO struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

type watchRoomDTO struct {
	ID           string           `json:"id"`
	Revision     int64            `json:"revision"`
	Generation   int64            `json:"generation"`
	State        string           `json:"state"`
	PositionMS   int64            `json:"position_ms"`
	AtMS         int64            `json:"at_ms"`
	Rate         float64          `json:"rate"`
	ServerTimeMS int64            `json:"server_time_ms"`
	WaitUntilMS  int64            `json:"wait_until_ms,omitempty"`
	HostID       string           `json:"host_id"`
	SelfID       string           `json:"self_id"`
	ChangedBy    string           `json:"changed_by,omitempty"`
	Available    bool             `json:"available"`
	ItemID       string           `json:"item_id,omitempty"`
	Item         *watchItemDTO    `json:"item,omitempty"`
	Participants []watchMemberDTO `json:"participants"`
}
type watchInviteDTO struct {
	Room              watchRoomDTO `json:"room"`
	InviteCode        string       `json:"invite_code"`
	InviteExpiresAtMS int64        `json:"invite_expires_at_ms"`
}
type watchConflictDTO struct {
	Error string       `json:"error"`
	Room  watchRoomDTO `json:"room"`
}

func decodeWatch(w http.ResponseWriter, r *http.Request, v any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return watchtogether.ErrInput
	}
	return nil
}
func (s *Server) watchActor(r *http.Request) (watchtogether.Actor, *store.User, error) {
	se := s.session(r)
	if se == nil || se.UserID == "" || se.UserID != currentUserID(r) {
		return watchtogether.Actor{}, nil, errWatchAuth
	}
	u, err := s.store.GetUser(r.Context(), se.UserID)
	if err != nil {
		return watchtogether.Actor{}, nil, err
	}
	if u == nil {
		return watchtogether.Actor{}, nil, errWatchAuth
	}
	return watchtogether.Actor{Session: se.TokenHash, UserID: u.ID, Name: u.Name}, u, nil
}
func userAccess(u *store.User) store.Access {
	if u.IsAdmin {
		return store.Access{}
	}
	return store.Access{MaxAge: u.MaxAge, Libraries: u.LibraryIDs, ExcludeLabels: u.ExcludeLabels}
}
func (s *Server) watchItem(ctx context.Context, id string, u *store.User) (*store.Item, error) {
	visible, err := s.store.QueueItems(ctx, []string{id}, userAccess(u))
	if err != nil {
		return nil, err
	}
	it, ok := visible[id]
	if !ok || (it.Kind != "movie" && it.Kind != "episode") {
		return nil, nil
	}
	return &it, nil
}
func (s *Server) watchView(r *http.Request, room watchtogether.Snapshot) (watchRoomDTO, error) {
	actor, u, err := s.watchActor(r)
	if err != nil {
		return watchRoomDTO{}, err
	}
	out := watchRoomDTO{ID: room.ID, Revision: room.Revision, Generation: room.Generation, State: room.State, PositionMS: room.PositionMS, AtMS: room.AtMS, Rate: room.Rate, ServerTimeMS: time.Now().UnixMilli(), WaitUntilMS: room.WaitUntilMS, HostID: room.HostID, ChangedBy: room.ChangedBy, Participants: []watchMemberDTO{}}
	for _, p := range room.Members {
		if p.Actor.Key() == actor.Key() {
			out.SelfID = p.ID
		}
		out.Participants = append(out.Participants, watchMemberDTO{ID: p.ID, Name: p.Actor.Name, Available: p.Available, Ready: p.Ready, State: p.State})
	}
	if out.SelfID == "" {
		return watchRoomDTO{}, watchtogether.ErrNotFound
	}
	it, err := s.watchItem(r.Context(), room.ItemID, u)
	if err != nil {
		return watchRoomDTO{}, err
	}
	out.Available = it != nil
	if it != nil {
		d := watchItemDTO{ID: it.ID, Title: it.Title, Kind: it.Kind}
		out.ItemID = it.ID
		out.Item = &d
	}
	// The requester's fresh access decision overrides any older presence report in this snapshot.
	for i := range out.Participants {
		if out.Participants[i].ID == out.SelfID {
			out.Participants[i].Available = out.Available
			if !out.Available {
				out.Participants[i].Ready = false
			}
		}
	}
	return out, nil
}
func (s *Server) watchFailure(w http.ResponseWriter, r *http.Request, room watchtogether.Snapshot, err error) {
	switch {
	case errors.Is(err, errWatchAuth):
		writeError(w, 401, "sign in and choose the same profile again")
	case errors.Is(err, watchtogether.ErrStale):
		if room.ID == "" {
			writeError(w, 409, "room changed; retry")
			return
		}
		d, e := s.watchView(r, room)
		if e != nil {
			s.watchFailure(w, r, watchtogether.Snapshot{}, e)
			return
		}
		writeJSON(w, 409, watchConflictDTO{Error: "room changed", Room: d})
	case errors.Is(err, watchtogether.ErrNotFound):
		writeError(w, 404, "room or playable invitation not found")
	case errors.Is(err, watchtogether.ErrForbidden):
		writeError(w, 403, err.Error())
	case errors.Is(err, watchtogether.ErrInput):
		writeError(w, 400, "invalid room request")
	case errors.Is(err, watchtogether.ErrLimit):
		writeError(w, 429, "room, participant or stream limit reached")
	case errors.Is(err, watchtogether.ErrRate):
		w.Header().Set("Retry-After", "1")
		writeError(w, 429, "room updates are too frequent")
	default:
		s.fail(w, err)
	}
}

// watchRoom refreshes the caller's membership/access without revealing a room to nonmembers.
func (s *Server) watchRoom(w http.ResponseWriter, r *http.Request) (watchtogether.Snapshot, watchtogether.Actor, bool) {
	actor, u, err := s.watchActor(r)
	if err != nil {
		s.watchFailure(w, r, watchtogether.Snapshot{}, err)
		return watchtogether.Snapshot{}, actor, false
	}
	for range 3 {
		room, _, e := s.watchTogether.Get(r.PathValue("id"), actor.Key())
		if e != nil {
			s.watchFailure(w, r, room, e)
			return room, actor, false
		}
		it, e := s.watchItem(r.Context(), room.ItemID, u)
		if e != nil {
			s.watchFailure(w, r, room, e)
			return room, actor, false
		}
		room, e = s.watchTogether.SetAccess(room.ID, actor.Key(), room.ItemID, it != nil)
		if errors.Is(e, watchtogether.ErrStale) {
			continue
		}
		if e != nil {
			s.watchFailure(w, r, room, e)
			return room, actor, false
		}
		return room, actor, true
	}
	s.watchFailure(w, r, watchtogether.Snapshot{}, watchtogether.ErrStale)
	return watchtogether.Snapshot{}, actor, false
}
func (s *Server) writeWatch(w http.ResponseWriter, r *http.Request, room watchtogether.Snapshot) {
	d, err := s.watchView(r, room)
	if err != nil {
		s.watchFailure(w, r, room, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, d)
}

// createWatchTogether: POST /api/watch-together {item_id,start_ms} creates a paused room and an
// expiring readable invite code. Up to 20 rooms/server, 10 participants/room, one active room per
// device/profile. Existing signed-in household profiles and friends are allowed. No stream starts.
func (s *Server) createWatchTogether(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ItemID  string `json:"item_id"`
		StartMS int64  `json:"start_ms" api:"optional"`
	}
	if decodeWatch(w, r, &body) != nil || body.StartMS < 0 || body.StartMS > watchtogether.MaxPositionMS {
		writeError(w, 400, "send item_id and optional nonnegative start_ms")
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
	room, code, expiry, err := s.watchTogether.Create(a, it.ID, body.StartMS)
	if err != nil {
		s.watchFailure(w, r, room, err)
		return
	}
	d, err := s.watchView(r, room)
	if err != nil {
		s.watchFailure(w, r, room, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 201, watchInviteDTO{Room: d, InviteCode: code, InviteExpiresAtMS: expiry.UnixMilli()})
}

// watchAdmission uses the existing bounded-attempt authentication limiter: five create/join
// requests per minute/device, in addition to the invite-guess limit per address below.
func (s *Server) watchAdmission(w http.ResponseWriter, r *http.Request, a watchtogether.Actor) bool {
	key := "watch-admit:" + a.Session
	if !s.limiter.Attempt(key) {
		w.Header().Set("Retry-After", "60")
		writeError(w, 429, "try creating or joining a room again in a minute")
		return false
	}
	return true
}

// joinWatchTogether: POST /api/watch-together/join {code} joins an unexpired invite only if the
// profile may play the current movie/episode. Wrong, expired and restricted invites all return 404.
func (s *Server) joinWatchTogether(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if decodeWatch(w, r, &body) != nil {
		writeError(w, 400, "send an invite code")
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
	key := "watch-code:" + clientIP(r)
	if !s.limiter.Attempt(key) {
		w.Header().Set("Retry-After", "60")
		writeError(w, 429, "too many invite attempts")
		return
	}
	item, err := s.watchTogether.InviteItem(body.Code)
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
	room, err := s.watchTogether.Join(a, body.Code, item)
	if err != nil {
		s.watchFailure(w, r, watchtogether.Snapshot{}, err)
		return
	}
	s.limiter.Succeed(key)
	s.writeWatch(w, r, room)
}

// getWatchTogether: GET /api/watch-together/{id} returns a member-only personalized timeline.
// A member denied a changed title remains present with available:false and no item/id metadata.
func (s *Server) getWatchTogether(w http.ResponseWriter, r *http.Request) {
	room, _, ok := s.watchRoom(w, r)
	if ok {
		s.writeWatch(w, r, room)
	}
}

// leaveWatchTogether: POST /api/watch-together/{id}/leave leaves this device/profile only.
// The oldest remaining participant inherits host control. Personal playback/history is untouched.
func (s *Server) leaveWatchTogether(w http.ResponseWriter, r *http.Request) {
	room, a, ok := s.watchRoom(w, r)
	if !ok {
		return
	}
	if err := s.watchTogether.Leave(room.ID, a.Key(), "left"); err != nil {
		s.watchFailure(w, r, room, err)
		return
	}
	w.WriteHeader(204)
}

// commandWatchTogether: POST /api/watch-together/{id}/commands {revision,action,position_ms,item_id}.
// Actions play/pause/seek/change_item are shared controls; only the host may end. Seek and item
// changes increment generation so readiness for older media/positions cannot release the barrier.
// A stale revision returns 409 with the current personalized room. No playback session is created.
func (s *Server) commandWatchTogether(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Revision   int64  `json:"revision"`
		Action     string `json:"action"`
		PositionMS int64  `json:"position_ms" api:"optional"`
		ItemID     string `json:"item_id" api:"optional"`
	}
	if decodeWatch(w, r, &body) != nil || body.Revision < 1 {
		writeError(w, 400, "send revision and action")
		return
	}
	room, a, ok := s.watchRoom(w, r)
	if !ok {
		return
	}
	allowed := map[string]bool{}
	if body.Action == "change_item" {
		for _, p := range room.Members {
			se, err := s.store.GetSession(r.Context(), p.Actor.Session)
			if err != nil {
				s.fail(w, err)
				return
			}
			if se == nil || se.UserID != p.Actor.UserID {
				continue
			}
			u, err := s.store.GetUser(r.Context(), p.Actor.UserID)
			if err != nil {
				s.fail(w, err)
				return
			}
			if u == nil {
				continue
			}
			it, err := s.watchItem(r.Context(), body.ItemID, u)
			if err != nil {
				s.fail(w, err)
				return
			}
			allowed[p.Actor.Key()] = it != nil
		}
		if !allowed[a.Key()] {
			writeError(w, 404, "playable movie or episode not found")
			return
		}
	}
	next, err := s.watchTogether.Command(room.ID, a.Key(), body.Revision, body.Action, body.ItemID, body.PositionMS, allowed)
	if err != nil {
		s.watchFailure(w, r, next, err)
		return
	}
	s.writeWatch(w, r, next)
}

// heartbeatWatchTogether: POST /api/watch-together/{id}/heartbeat
// {generation,position_ms,state,ready} reports presence/readiness, never changes shared position or
// watch history. Up to four reports/second. Waiting defaults to ten seconds, then slow viewers catch
// up themselves. A stale seek/title generation returns 409 with current room state.
func (s *Server) heartbeatWatchTogether(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Generation int64  `json:"generation"`
		PositionMS int64  `json:"position_ms"`
		State      string `json:"state"`
		Ready      bool   `json:"ready"`
	}
	if decodeWatch(w, r, &body) != nil {
		writeError(w, 400, "send generation, position_ms, state and ready")
		return
	}
	room, a, ok := s.watchRoom(w, r)
	if !ok {
		return
	}
	next, err := s.watchTogether.Heartbeat(room.ID, a.Key(), body.Generation, body.PositionMS, body.State, body.Ready)
	if err != nil {
		s.watchFailure(w, r, next, err)
		return
	}
	s.writeWatch(w, r, next)
}

// renewWatchInvite: POST /api/watch-together/{id}/invite {revision} creates a fresh 15-minute
// code (host only), invalidating the old one. Codes are returned only at create/renew, never in GET.
func (s *Server) renewWatchInvite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Revision int64 `json:"revision"`
	}
	if decodeWatch(w, r, &body) != nil {
		writeError(w, 400, "send revision")
		return
	}
	room, a, ok := s.watchRoom(w, r)
	if !ok {
		return
	}
	next, code, expiry, err := s.watchTogether.RenewInvite(room.ID, a.Key(), body.Revision)
	if err != nil {
		s.watchFailure(w, r, next, err)
		return
	}
	d, err := s.watchView(r, next)
	if err != nil {
		s.watchFailure(w, r, next, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, watchInviteDTO{Room: d, InviteCode: code, InviteExpiresAtMS: expiry.UnixMilli()})
}

// watchTogetherTime: GET /api/watch-together/time?client_sent_ms=... echoes an optional integer
// client timestamp and server receive/send epoch milliseconds for midpoint/RTT clock estimation.
func (s *Server) watchTogetherTime(w http.ResponseWriter, r *http.Request) {
	received := time.Now().UnixMilli()
	var client int64
	if raw := r.URL.Query().Get("client_sent_ms"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			writeError(w, 400, "invalid client_sent_ms")
			return
		}
		client = n
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, struct {
		ClientSentMS     int64 `json:"client_sent_ms"`
		ServerReceivedMS int64 `json:"server_received_ms"`
		ServerSentMS     int64 `json:"server_sent_ms"`
	}{client, received, time.Now().UnixMilli()})
}

// currentWatchTogether: GET /api/watch-together/current recovers only this device/profile's own
// active membership (404 if none). Use after reload or a lost create/join response; never lists rooms.
func (s *Server) currentWatchTogether(w http.ResponseWriter, r *http.Request) {
	a, _, err := s.watchActor(r)
	if err != nil {
		s.watchFailure(w, r, watchtogether.Snapshot{}, err)
		return
	}
	room, err := s.watchTogether.Current(a.Key())
	if err != nil {
		s.watchFailure(w, r, room, err)
		return
	}
	r.SetPathValue("id", room.ID)
	s.getWatchTogether(w, r)
}

package api

import (
	"ams/internal/watchtogether"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// watchTogetherEvents: GET /api/watch-together/{id}/events is a private room SSE stream carrying
// full personalized room snapshots (event: room, id: revision). It does not consume /events slots.
// One stream/member (a reconnect replaces the older one), at most 200/server; initial/reconnect always sends the latest snapshot.
// Changes wake immediately; timeout transitions are checked every 200ms. Auth/access are freshly
// checked before each payload and at least every two seconds; revoked/switched sessions close.
// No tokens/codes in URLs. Thirty-minute streams, five-second write deadlines, no event queues.
func (s *Server) watchTogetherEvents(w http.ResponseWriter, r *http.Request) {
	room, a, ok := s.watchRoom(w, r)
	if !ok {
		return
	}
	stream, err := s.watchTogether.OpenStream(room.ID, a.Key())
	if err != nil {
		s.watchFailure(w, r, room, err)
		return
	}
	defer s.watchTogether.CloseStream(room.ID, a.Key(), stream.Token)
	self := stream.MemberID
	controller := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	defer controller.SetWriteDeadline(time.Time{})
	send := func(snapshot watchtogether.Snapshot) bool {
		d, err := s.watchView(r, snapshot)
		if err != nil {
			return false
		}
		if d.SelfID != self {
			return false
		}
		b, err := json.Marshal(d)
		if err != nil {
			return false
		}
		_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err = fmt.Fprintf(w, "id: %d\nevent: room\ndata: %s\n\n", d.Revision, b); err != nil {
			return false
		}
		return controller.Flush() == nil
	}
	if !send(room) || room.State == "ended" {
		return
	}
	revision := room.Revision
	check := s.watchRecheck
	if check <= 0 {
		check = 2 * time.Second
	}
	authTick := time.NewTicker(check)
	defer authTick.Stop()
	ticks := time.NewTicker(200 * time.Millisecond)
	defer ticks.Stop()
	lifetime := time.NewTimer(30 * time.Minute)
	defer lifetime.Stop()
	for {
		raw, changed, err := s.watchTogether.Get(room.ID, a.Key())
		if err != nil {
			return
		}
		if raw.Revision != revision {
			if !send(raw) {
				return
			}
			revision = raw.Revision
			if raw.State == "ended" {
				return
			}
		}
		select {
		case <-r.Context().Done():
			return
		case <-shuttingDown(r):
			return // Arc is restarting: the app reconnects
		case <-stream.Stop:
			// A newer stream for this member (a reconnect) replaced this one.
			return
		case <-lifetime.C:
			return
		case <-changed:
		case <-ticks.C:
		case <-authTick.C:
			actor, u, err := s.watchActor(r)
			if err != nil || actor.Key() != a.Key() {
				s.watchTogether.Leave(room.ID, a.Key(), "session revoked or switched")
				return
			}
			raw, _, err = s.watchTogether.Get(room.ID, a.Key())
			if err != nil {
				return
			}
			it, err := s.watchItem(r.Context(), raw.ItemID, u)
			if err != nil {
				return
			}
			raw, err = s.watchTogether.SetAccess(raw.ID, a.Key(), raw.ItemID, it != nil)
			if err != nil {
				continue
			}
			if err = s.watchTogether.Touch(raw.ID, a.Key()); err != nil {
				return
			}
			if !send(raw) {
				return
			}
			revision = raw.Revision
			if raw.State == "ended" {
				return
			}
		}
	}
}

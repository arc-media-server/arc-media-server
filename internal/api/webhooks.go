package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"ams/internal/notify"
	"ams/internal/store"
)

type webhookDTO struct {
	store.Webhook
	// How its last delivery went (nil before the first).
	Last *notify.Delivery `json:"last,omitempty"`
	// The kind of address: discord, slack, or json for anything else.
	Format string `json:"format"`
}

func webhookFormat(u string) string {
	switch {
	case strings.Contains(u, "discord.com/api/webhooks") || strings.Contains(u, "discordapp.com/api/webhooks"):
		return "discord"
	case strings.Contains(u, "hooks.slack.com"):
		return "slack"
	}
	return "json"
}

// listWebhooks: GET /api/webhooks (admin) lists webhooks, how each last went, and every event.
func (s *Server) listWebhooks(w http.ResponseWriter, r *http.Request) {
	hooks, err := s.store.Webhooks(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	last := s.notify.Last()
	out := make([]webhookDTO, 0, len(hooks))
	for _, h := range hooks {
		d := webhookDTO{Webhook: h, Format: webhookFormat(h.URL)}
		if l, ok := last[h.ID]; ok {
			d.Last = &l
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": out, "events": notify.Events})
}

// saveWebhook: POST /api/webhooks or PUT /api/webhooks/{id} {"name", "url", "events"} (admin).
func (s *Server) saveWebhook(w http.ResponseWriter, r *http.Request) {
	var h store.Webhook
	if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	h.ID = r.PathValue("id")
	h.URL, h.Name = strings.TrimSpace(h.URL), strings.TrimSpace(h.Name)
	if u, err := url.Parse(h.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeError(w, http.StatusBadRequest, "enter the webhook's full address, starting with https://")
		return
	}
	for _, e := range h.Events {
		if !slices.Contains(notify.Events, e) {
			writeError(w, http.StatusBadRequest, "unknown event: "+e)
			return
		}
	}
	if h.Name == "" {
		h.Name = map[string]string{"discord": "Discord", "slack": "Slack", "json": "Webhook"}[webhookFormat(h.URL)]
	}
	if err := s.store.SaveWebhook(r.Context(), &h); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, webhookDTO{Webhook: h, Format: webhookFormat(h.URL)})
}

// deleteWebhook: DELETE /api/webhooks/{id} (admin).
func (s *Server) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteWebhook(r.Context(), r.PathValue("id")); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// testWebhook: POST /api/webhooks/{id}/test (admin) sends a test message now and says how it went.
func (s *Server) testWebhook(w http.ResponseWriter, r *http.Request) {
	hooks, err := s.store.Webhooks(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	for _, h := range hooks {
		if h.ID == r.PathValue("id") {
			writeJSON(w, http.StatusOK, s.notify.Test(r.Context(), h))
			return
		}
	}
	writeError(w, http.StatusNotFound, "no such webhook")
}

// news: GET /api/news lists what's new for the profile: new episodes of shows it watches, and new
// movies it hasn't seen, from the last month, with how many arrived since it last looked.
func (s *Server) news(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := currentUserID(r)
	since := time.Now().AddDate(0, 0, -30).Unix()
	news, err := s.store.News(ctx, userID, access(r), since, 40)
	if err != nil {
		s.fail(w, err)
		return
	}
	seen, err := s.store.NewsSeen(ctx, userID)
	if err != nil {
		s.fail(w, err)
		return
	}
	type entry struct {
		Item   itemDTO `json:"item"`
		Series *struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"series,omitempty"`
		// For a show: how many new episodes.
		Count   int   `json:"count"`
		AddedAt int64 `json:"added_at"`
		Unread  bool  `json:"unread"`
	}
	out := []entry{}
	unread := 0
	for _, n := range news {
		e := entry{Item: toDTO(n.Item), Count: n.Count, AddedAt: n.AddedAt, Unread: n.AddedAt > seen}
		if n.Item.Kind == "series" {
			e.Series = &struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			}{n.Item.ID, n.Item.Title}
		}
		if e.Unread {
			unread++
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out, "unread": unread})
}

// newsSeen: POST /api/news/seen marks everything new as seen by the profile.
func (s *Server) newsSeen(w http.ResponseWriter, r *http.Request) {
	if err := s.store.SetNewsSeen(r.Context(), currentUserID(r), time.Now().Unix()); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// theme: GET /api/items/{id}/theme plays a show's or movie's theme tune.
func (s *Server) theme(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	if it.LocalTheme == "" {
		writeError(w, http.StatusNotFound, "no theme music")
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeFile(w, r, it.LocalTheme)
}

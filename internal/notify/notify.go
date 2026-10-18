// Package notify sends events (new titles, playback, problems) to webhooks: any URL as JSON, or
// Discord and Slack in their own message formats.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"ams/internal/store"
)

// Event types.
const (
	LibraryNew    = "library.new"      // movies or episodes were added
	PlaybackStart = "playback.start"   // someone has been watching for a minute
	PlaybackDone  = "playback.watched" // someone watched to the end
	ServerProblem = "server.problem"   // e.g. a backup failed
)

// Events lists every event, in the order the Settings page shows them.
var Events = []string{LibraryNew, PlaybackStart, PlaybackDone, ServerProblem}

// Item is the title an event is about.
type Item struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Year        int    `json:"year,omitempty"`
	SeriesID    string `json:"series_id,omitempty"`
	SeriesTitle string `json:"series_title,omitempty"`
	Season      int    `json:"season,omitempty"`
	Episode     int    `json:"episode,omitempty"`
	// Where to see it in Arc, and its picture (when the server address is known).
	URL   string `json:"url,omitempty"`
	Image string `json:"image,omitempty"`
}

// Event is what a webhook receives.
type Event struct {
	Event   string    `json:"event"`
	Time    time.Time `json:"time"`
	Server  string    `json:"server"`
	Message string    `json:"message"`
	Profile string    `json:"profile,omitempty"`
	Library string    `json:"library,omitempty"`
	Item    *Item     `json:"item,omitempty"`
	// library.new: how many titles were added, when it's more than the one item.
	Count int `json:"count,omitempty"`
}

// Delivery is how a webhook's last send went, for the Settings page.
type Delivery struct {
	At     time.Time `json:"at"`
	Event  string    `json:"event"`
	Status string    `json:"status"` // e.g. "204 No Content", or the error
	OK     bool      `json:"ok"`
}

// Notifier delivers events to the webhooks that want them.
type Notifier struct {
	store *store.Store
	log   *slog.Logger
	http  *http.Client
	// BaseURL, if set, makes links and pictures in messages (e.g. "http://192.168.1.10:8099").
	BaseURL func() string

	mu   sync.Mutex
	last map[string]Delivery
}

func New(st *store.Store, log *slog.Logger) *Notifier {
	return &Notifier{store: st, log: log, http: &http.Client{Timeout: 15 * time.Second}, last: map[string]Delivery{}}
}

// Last returns how each webhook's last delivery went, by id.
func (n *Notifier) Last() map[string]Delivery {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make(map[string]Delivery, len(n.last))
	for k, v := range n.last {
		out[k] = v
	}
	return out
}

// Send delivers an event, in the background, to every webhook that wants it.
func (n *Notifier) Send(ev Event) {
	if n == nil {
		return
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	ev.Server = "Arc Media Server"
	if ev.Item != nil && n.BaseURL != nil {
		if base := strings.TrimRight(n.BaseURL(), "/"); base != "" {
			ev.Item.URL = base + itemPath(ev.Item)
			pic := ev.Item.ID
			if ev.Item.SeriesID != "" {
				pic = ev.Item.SeriesID // an episode shows its show's poster
			}
			ev.Item.Image = base + "/api/items/" + pic + "/images/primary"
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		hooks, err := n.store.Webhooks(ctx)
		if err != nil {
			n.log.Warn("listing webhooks failed", "err", err)
			return
		}
		for _, h := range hooks {
			if h.Wants(ev.Event) {
				n.deliver(ctx, h, ev)
			}
		}
	}()
}

// Test sends a test event to one webhook straight away and says how it went.
func (n *Notifier) Test(ctx context.Context, h store.Webhook) Delivery {
	return n.deliver(ctx, h, Event{Event: "test", Time: time.Now(), Server: "Arc Media Server",
		Message: "This is a test from Arc Media Server. Webhooks are working."})
}

func itemPath(it *Item) string {
	switch it.Kind {
	case "series":
		return "/series/" + it.ID
	case "episode":
		return "/series/" + it.SeriesID
	}
	return "/movie/" + it.ID
}

// deliver posts an event to a webhook, trying twice more if the first fails.
func (n *Notifier) deliver(ctx context.Context, h store.Webhook, ev Event) Delivery {
	body, err := json.Marshal(payload(h.URL, ev))
	d := Delivery{At: time.Now(), Event: ev.Event}
	for attempt := 0; err == nil && attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				err = ctx.Err()
				continue
			case <-time.After(time.Duration(attempt*5) * time.Second):
			}
		}
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(body))
		if err != nil {
			break
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "ArcMediaServer")
		var resp *http.Response
		resp, err = n.http.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		d.Status = resp.Status
		if resp.StatusCode/100 == 2 {
			d.OK = true
			break
		}
		err = fmt.Errorf("%s", resp.Status)
		if resp.StatusCode/100 == 4 && resp.StatusCode != http.StatusTooManyRequests {
			break // the request itself is wrong; trying again won't help
		}
	}
	if err != nil {
		d.Status, d.OK = err.Error(), false
		n.log.Warn("webhook failed", "webhook", h.Name, "event", ev.Event, "err", err)
	}
	n.mu.Lock()
	n.last[h.ID] = d
	n.mu.Unlock()
	return d
}

// payload is the body for a webhook: Discord and Slack get a message in their own format, and
// anything else the event as JSON.
func payload(url string, ev Event) any {
	switch {
	case strings.Contains(url, "discord.com/api/webhooks") || strings.Contains(url, "discordapp.com/api/webhooks"):
		embed := map[string]any{"title": ev.Message, "timestamp": ev.Time.Format(time.RFC3339), "color": 0x8b5cf6,
			"footer": map[string]string{"text": "Arc Media Server"}}
		if ev.Item != nil {
			if ev.Item.URL != "" {
				embed["url"] = ev.Item.URL
			}
			if ev.Item.Image != "" && !strings.Contains(ev.Item.Image, "://127.") && !strings.Contains(ev.Item.Image, "://localhost") {
				embed["thumbnail"] = map[string]string{"url": ev.Item.Image}
			}
		}
		return map[string]any{"username": "Arc", "embeds": []any{embed}}
	case strings.Contains(url, "hooks.slack.com"):
		text := ev.Message
		if ev.Item != nil && ev.Item.URL != "" {
			text += " <" + ev.Item.URL + "|Open in Arc>"
		}
		return map[string]string{"text": text}
	}
	return ev
}

// LibraryScanned announces the movies and episodes a scan added (those first seen since a time):
// each movie, and each show's new episodes together. Lots at once (a new library) are one summary.
func (n *Notifier) LibraryScanned(ctx context.Context, lib store.Library, since int64) {
	if n == nil {
		return
	}
	hooks, err := n.store.Webhooks(ctx)
	if err != nil || len(hooks) == 0 {
		return
	}
	added, err := n.store.AddedSince(ctx, lib.ID, since, 51)
	if err != nil || len(added) == 0 {
		return
	}
	if len(added) > 50 {
		n.Send(Event{Event: LibraryNew, Library: lib.Name, Count: len(added),
			Message: fmt.Sprintf("Lots of new titles in %s", lib.Name)})
		return
	}
	episodes := map[string][]store.Item{}
	var shows []string
	for _, it := range added {
		if it.Kind == "movie" {
			title := it.Title
			if it.Year > 0 {
				title = fmt.Sprintf("%s (%d)", it.Title, it.Year)
			}
			what := "New movie: "
			if lib.Kind == "videos" {
				what, title = "New video: ", it.Title
			}
			n.Send(Event{Event: LibraryNew, Library: lib.Name, Message: what + title,
				Item: &Item{ID: it.ID, Kind: it.Kind, Title: it.Title, Year: it.Year}})
			continue
		}
		if _, ok := episodes[it.ParentID]; !ok {
			shows = append(shows, it.ParentID)
		}
		episodes[it.ParentID] = append(episodes[it.ParentID], it)
	}
	for _, sid := range shows {
		eps := episodes[sid]
		show, err := n.store.GetItem(ctx, sid)
		if err != nil || show == nil {
			continue
		}
		ev := Event{Event: LibraryNew, Library: lib.Name, Count: len(eps),
			Item: &Item{ID: eps[0].ID, Kind: "episode", Title: eps[0].Title, SeriesID: show.ID, SeriesTitle: show.Title}}
		if len(eps) == 1 {
			ev.Message = fmt.Sprintf("New episode of %s: %s", show.Title, eps[0].Title)
		} else {
			ev.Message = fmt.Sprintf("%d new episodes of %s", len(eps), show.Title)
		}
		n.Send(ev)
	}
}

// Playback announces that a profile started (playback.start) or finished (playback.watched)
// watching a movie or episode.
func (n *Notifier) Playback(ctx context.Context, event, profile string, it *store.Item) {
	if n == nil || it == nil {
		return
	}
	item := &Item{ID: it.ID, Kind: it.Kind, Title: it.Title, Year: it.Year}
	name := it.Title
	if it.Kind == "episode" {
		if show, err := n.store.GetItem(ctx, it.ParentID); err == nil && show != nil {
			item.SeriesID, item.SeriesTitle = show.ID, show.Title
			name = show.Title + ": " + it.Title
		}
	}
	verb := "is watching"
	if event == PlaybackDone {
		verb = "watched"
	}
	n.Send(Event{Event: event, Profile: profile, Item: item, Message: fmt.Sprintf("%s %s %s", profile, verb, name)})
}

// Problem announces something that went wrong and needs a look.
func (n *Notifier) Problem(message string) {
	n.Send(Event{Event: ServerProblem, Message: message})
}

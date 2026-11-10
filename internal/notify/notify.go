// Package notify sends events (new titles, playback, problems) to webhooks: any URL as JSON, or
// Discord and Slack in their own message formats.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	store     *store.Store
	log       *slog.Logger
	http      *http.Client
	waitRetry func(context.Context, time.Duration) error
	// BaseURL, if set, makes links and pictures in messages (e.g. "http://192.168.1.10:8099").
	BaseURL func() string

	ctx     context.Context
	cancel  context.CancelFunc
	closed  bool
	workers sync.WaitGroup
	mu      sync.Mutex
	last    map[string]Delivery
}

func New(st *store.Store, log *slog.Logger) *Notifier {
	return NewWithContext(context.Background(), st, log)
}

// NewWithContext owns asynchronous deliveries until process shutdown.
func NewWithContext(ctx context.Context, st *store.Store, log *slog.Logger) *Notifier {
	ctx, cancel := context.WithCancel(ctx)
	return &Notifier{ctx: ctx, cancel: cancel, store: st, log: log, http: &http.Client{Timeout: 15 * time.Second}, waitRetry: waitWebhookRetry, last: map[string]Delivery{}}
}

// Close cancels deliveries and joins every admitted store/network operation.
func (n *Notifier) Close() {
	if n == nil {
		return
	}
	n.mu.Lock()
	n.closed = true
	n.cancel()
	n.mu.Unlock()
	n.workers.Wait()
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
	n.mu.Lock()
	if n.closed || n.ctx.Err() != nil {
		n.mu.Unlock()
		return
	}
	n.workers.Add(1)
	n.mu.Unlock()
	go func() {
		defer n.workers.Done()
		ctx, cancel := context.WithTimeout(n.ctx, time.Minute)
		defer cancel()
		hooks, err := n.store.Webhooks(ctx)
		if err != nil {
			if ctx.Err() == nil {
				n.log.Warn("listing webhooks failed", "err", err)
			}
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
	attempts := 0
	started := time.Now()
	if err != nil {
		err = errors.New("webhook payload could not be encoded")
	} else {
		for attempt := 0; attempt < 3; attempt++ {
			if ctx.Err() != nil {
				err = ctx.Err()
				break
			}
			if attempt > 0 {
				wait := n.waitRetry
				if wait == nil {
					wait = waitWebhookRetry
				}
				if err = wait(ctx, time.Duration(attempt*5)*time.Second); err != nil {
					break
				}
			}
			attempts++
			req, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(body))
			if requestErr != nil {
				err = errors.New("invalid webhook URL")
				break
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "ArcMediaServer")
			var resp *http.Response
			resp, err = n.http.Do(req)
			if err != nil {
				if ctx.Err() != nil {
					err = ctx.Err()
					break
				}
				if errors.Is(err, context.DeadlineExceeded) {
					err = fmt.Errorf("webhook request timed out: %w", context.DeadlineExceeded)
				} else {
					err = errors.New("webhook transport failed")
				}
				continue
			}
			resp.Body.Close()
			d.Status = fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
			if resp.StatusCode/100 == 2 {
				d.OK = true
				err = nil
				break
			}
			err = errors.New(d.Status)
			if resp.StatusCode/100 == 4 && resp.StatusCode != http.StatusTooManyRequests {
				break
			}
		}
	}

	if err != nil {
		d.Status, d.OK = err.Error(), false
		if ctx.Err() == nil {
			n.log.Warn("webhook failed", "webhook", h.Name, "event", ev.Event, "attempts", attempts, "took", time.Since(started), "err", err)
		}
	}
	n.mu.Lock()
	n.last[h.ID] = d
	n.mu.Unlock()
	return d
}

// waitWebhookRetry is canceled promptly when delivery or server work ends.
func waitWebhookRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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

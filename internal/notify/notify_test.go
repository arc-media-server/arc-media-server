package notify

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ams/internal/store"
)

func TestPayloads(t *testing.T) {
	ev := Event{Event: LibraryNew, Message: "New movie: Heat (1995)", Item: &Item{ID: "x", URL: "http://192.168.1.2:8099/movie/x",
		Image: "http://192.168.1.2:8099/api/items/x/images/primary"}}
	d, _ := json.Marshal(payload("https://discord.com/api/webhooks/1/abc", ev))
	var discord struct {
		Embeds []struct {
			Title     string            `json:"title"`
			URL       string            `json:"url"`
			Thumbnail map[string]string `json:"thumbnail"`
		} `json:"embeds"`
	}
	json.Unmarshal(d, &discord)
	if len(discord.Embeds) != 1 || discord.Embeds[0].Title != ev.Message || discord.Embeds[0].URL == "" {
		t.Errorf("discord = %s", d)
	}
	s, _ := json.Marshal(payload("https://hooks.slack.com/services/x", ev))
	var slack struct{ Text string }
	json.Unmarshal(s, &slack)
	if slack.Text != "New movie: Heat (1995) <http://192.168.1.2:8099/movie/x|Open in Arc>" {
		t.Errorf("slack = %s", s)
	}
	j, _ := json.Marshal(payload("https://example.com/hook", ev))
	var plain Event
	json.Unmarshal(j, &plain)
	if plain.Event != LibraryNew || plain.Item == nil || plain.Item.ID != "x" {
		t.Errorf("json = %s", j)
	}
}

func TestDeliver(t *testing.T) {
	got := make(chan Event, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ev Event
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &ev)
		got <- ev
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := store.Webhook{Name: "Test", URL: srv.URL, Events: []string{ServerProblem}}
	if err := st.SaveWebhook(ctx, &h); err != nil {
		t.Fatal(err)
	}
	n := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	n.Send(Event{Event: PlaybackStart, Message: "not wanted"})
	n.Problem("The automatic backup failed")
	if ev := <-got; ev.Event != ServerProblem || ev.Message != "The automatic backup failed" {
		t.Errorf("delivered %+v", ev)
	}
	if d := n.Test(ctx, h); !d.OK || d.Status != "204 No Content" {
		t.Errorf("test delivery = %+v", d)
	}
}

package main

import (
	"ams/internal/settings"
	"testing"
)

func TestPublicURLWebhookAddress(t *testing.T) {
	c := settings.Settings{PublicURL: "https://arc.example.com", RemoteAccess: true, Port: 8099}
	if got := publicURL(c); got != c.PublicURL {
		t.Fatal(got)
	}
	t.Setenv("AMS_LISTEN", "127.0.0.1:8099")
	c.RemoteAccess = false
	if got := publicURL(c); got != "http://127.0.0.1:8099" {
		t.Fatal("disabled remote URL changed local links", got)
	}
}

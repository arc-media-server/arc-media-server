package api

import (
	"ams/internal/auth"
	"ams/internal/store"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDownloadManifestIsPreparationSnapshot(t *testing.T) {
	x := downloadTransferFixture(t, false)
	title := "Changed after preparation"
	if err := x.s.store.EditItem(t.Context(), x.d.ItemID, store.ItemEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	res, body := x.request(t, "GET", "/manifest", "", "")
	var got struct{ Item struct{ Title string } }
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || got.Item.Title != "Fixture" {
		t.Fatalf("manifest drifted after ready: %d %s", res.StatusCode, body)
	}
}

func TestOfflineCaptionAssetDeliveryAndMissingBundle(t *testing.T) {
	for _, tls := range []bool{false, true} {
		t.Run(map[bool]string{false: "http1", true: "http2"}[tls], func(t *testing.T) {
			x := downloadTransferFixture(t, tls)
			stem := strings.TrimSuffix(x.d.SourcePath, filepath.Ext(x.d.SourcePath))
			caption := []byte("1\n00:00:01,000 --> 00:00:02,000\nOffline caption\n")
			if err := os.WriteFile(stem+".en.srt", caption, 0600); err != nil {
				t.Fatal(err)
			}
			bundleRoot := t.TempDir()
			x.m.ConfigureBundles(bundleRoot, t.TempDir(), nil, nil)
			source, err := x.s.store.DownloadSource(t.Context(), x.d)
			if err != nil {
				t.Fatal(err)
			}
			if err = x.s.store.DeleteDownload(t.Context(), x.d.UserID, x.d.SessionHash, x.d.ID); err != nil {
				t.Fatal(err)
			}
			x.d, _, err = x.s.store.CreateDownload(t.Context(), x.user, x.d.SessionHash, x.d.ItemID, "offline-bundle", *source)
			if err != nil {
				t.Fatal(err)
			}
			if err = x.m.Prepare(t.Context(), x.d); err != nil {
				t.Fatal(err)
			}
			x.d, err = x.s.store.Download(t.Context(), x.d.UserID, x.d.SessionHash, x.d.ID)
			if err != nil {
				t.Fatal(err)
			}
			res, body := x.request(t, "GET", "/manifest", "", "")
			if res.StatusCode != 200 {
				t.Fatalf("manifest %d %s", res.StatusCode, body)
			}
			var manifest downloadManifest
			if err = json.Unmarshal(body, &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.ProfileID != x.user {
				t.Fatal("profile binding missing")
			}
			var asset downloadAssetDTO
			for _, a := range manifest.Assets {
				if a.Role == "subtitle" {
					asset = a
				}
			}
			if asset.ID == "" {
				t.Fatal("caption missing")
			}
			res, body = x.request(t, "GET", "/assets/"+asset.ID, "", "")
			if res.StatusCode != 200 || string(body) != string(caption) {
				t.Fatalf("caption bytes: %d %q", res.StatusCode, body)
			}
			res, body = x.request(t, "GET", "/assets/"+asset.ID, "bytes=0-3", "")
			if res.StatusCode != 206 || string(body) != string(caption[:4]) {
				t.Fatal("derived range wrong")
			}
			// A database-only restore with no copies cannot invent saved assets.
			if err := os.RemoveAll(bundleRoot); err != nil {
				t.Fatal(err)
			}
			res, _ = x.request(t, "GET", "/manifest", "", "")
			if res.StatusCode != 409 {
				t.Fatalf("missing bundle %d", res.StatusCode)
			}
		})
	}
}

func TestDownloadDoesNotCreatePlaybackOrWatchState(t *testing.T) {
	x := downloadTransferFixture(t, false)
	before, err := x.s.store.UserDataFor(t.Context(), x.user, []string{x.d.ItemID})
	if err != nil {
		t.Fatal(err)
	}
	res, _ := x.request(t, "GET", "/assets/"+x.d.Revision, "", "")
	if res.StatusCode != 200 {
		t.Fatal(res.StatusCode)
	}
	after, err := x.s.store.UserDataFor(t.Context(), x.user, []string{x.d.ItemID})
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("download changed watch state", err)
	}
	admin, err := x.s.store.CreateUser(t.Context(), "Transfer inspector", true, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	token := "transfer-inspector"
	if err = x.s.store.CreateSession(t.Context(), auth.HashToken(token), admin.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", x.server.URL+"/api/v1/playing", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := x.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var active []json.RawMessage
	if response.StatusCode != 200 || json.Unmarshal(body, &active) != nil || len(active) != 0 {
		t.Fatalf("download registered playback: %d %s", response.StatusCode, body)
	}
}

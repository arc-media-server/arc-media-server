package api

import (
	"ams/internal/auth"
	"ams/internal/downloads"
	"ams/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type transferFixture struct {
	s      *Server
	m      *downloads.Manager
	d      *store.Download
	server *httptest.Server
	token  string
	data   []byte
	user   string
}

func downloadTransferFixture(t *testing.T, tls bool) *transferFixture {
	t.Helper()
	s, u := securityServer(t)
	ctx := t.Context()
	yes := true
	if err := s.store.UpdateUser(ctx, u.ID, store.UserUpdate{AllowDownloads: &yes}); err != nil {
		t.Fatal(err)
	}
	token := "transfer-fixture"
	session := auth.HashToken(token)
	if err := s.store.CreateSession(ctx, session, u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	lib, err := s.store.SyncLibrary(ctx, "Downloads", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "transfer")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte(strings.Repeat("0123456789", 100))
	path := filepath.Join(t.TempDir(), "private.mp4")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	f := store.MediaFile{LibraryID: lib, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix()}
	f.ID, err = s.store.SaveFile(ctx, &f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetFileItems(ctx, f.ID, []string{item}, 0); err != nil {
		t.Fatal(err)
	}
	d, _, err := s.store.CreateDownload(ctx, u.ID, session, item, "transfer", f)
	if err != nil {
		t.Fatal(err)
	}
	m := downloads.New(s.store, s.log)
	if err = m.Prepare(ctx, d); err != nil {
		t.Fatal(err)
	}
	d, err = s.store.Download(ctx, u.ID, session, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log, Downloads: m})
	srv := httptest.NewUnstartedServer(h)
	srv.EnableHTTP2 = tls
	if tls {
		srv.StartTLS()
	} else {
		srv.Start()
	}
	t.Cleanup(srv.Close)
	return &transferFixture{s: s, m: m, d: d, server: srv, token: token, data: data, user: u.ID}
}
func (x *transferFixture) request(t *testing.T, method, path, rangeHeader, ifRange string) (*http.Response, []byte) {
	t.Helper()
	r, err := http.NewRequest(method, x.server.URL+"/api/v1/downloads/"+x.d.ID+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+x.token)
	r.Header.Set("Range", rangeHeader)
	r.Header.Set("If-Range", ifRange)
	res, err := x.server.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, body
}
func TestDownloadManifestAndSingleRangeDelivery(t *testing.T) {
	for _, tls := range []bool{false, true} {
		t.Run(map[bool]string{false: "http1", true: "http2"}[tls], func(t *testing.T) {
			x := downloadTransferFixture(t, tls)
			res, body := x.request(t, "GET", "/manifest", "", "")
			if res.StatusCode != 200 {
				t.Fatalf("manifest %d %s", res.StatusCode, body)
			}
			var manifest struct {
				Revision string
				Assets   []struct {
					ID     string
					URL    string
					SHA256 string
				}
			}
			if err := json.Unmarshal(body, &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.Revision != x.d.Revision || len(manifest.Assets) != 1 || manifest.Assets[0].SHA256 != x.d.SHA256 {
				t.Fatalf("manifest: %s", body)
			}
			asset := "/assets/" + manifest.Assets[0].ID
			for _, tc := range []struct {
				method, rangeHeader, ifRange string
				status                       int
				want                         []byte
			}{{"GET", "", "", 200, x.data}, {"HEAD", "bytes=0-1", "", 200, nil}, {"GET", "bytes=2-4", "", 206, x.data[2:5]}, {"GET", "bytes=-3", "", 206, x.data[len(x.data)-3:]}, {"GET", "bytes=99999-", "", 416, nil}, {"GET", "bytes=bad", "", 416, nil}, {"GET", "bytes=0-1,4-5", "", 416, nil}, {"GET", "bytes=2-4", `"not-a-validator"`, 200, x.data}} {
				r, b := x.request(t, tc.method, asset, tc.rangeHeader, tc.ifRange)
				if r.StatusCode != tc.status {
					t.Fatalf("%s %q: %d %s", tc.method, tc.rangeHeader, r.StatusCode, b)
				}
				if tc.status != 416 && string(b) != string(tc.want) {
					t.Fatalf("wrong bytes for %q", tc.rangeHeader)
				}
				if r.Header.Get("ETag") != "" || r.Header.Get("Last-Modified") != "" {
					t.Fatal("mutable original emitted validator")
				}
				if tc.status == 416 && r.Header.Get("Content-Range") != "bytes */1000" {
					t.Fatal("missing range size")
				}
				if tls && r.ProtoMajor != 2 {
					t.Fatal("HTTP2 not exercised")
				}
			}
			for _, secret := range []string{x.d.SourcePath, x.d.SessionHash, x.d.Key, x.d.SourceCacheKey} {
				if strings.Contains(string(body), secret) {
					t.Fatal("manifest leaked private provenance")
				}
			}
		})
	}
}

func TestActiveDownloadRevocationAbortsHTTPStream(t *testing.T) {
	for _, tls := range []bool{false, true} {
		for _, change := range []string{"grant", "release", "session", "source"} {
			t.Run(fmt.Sprintf("http2=%v/%s", tls, change), func(t *testing.T) {
				x := downloadTransferFixture(t, tls)
				// A sparse generated file exceeds both transports' socket/flow-control buffers.
				f, err := os.OpenFile(x.d.SourcePath, os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				if err = f.Truncate(64 << 20); err != nil {
					t.Fatal(err)
				}
				f.Close()
				info, _ := os.Stat(x.d.SourcePath)
				old, err := x.s.store.DownloadSource(t.Context(), x.d)
				if err != nil {
					t.Fatal(err)
				}
				old.Size, old.ModTime = info.Size(), info.ModTime().Unix()
				if _, err = x.s.store.SaveFile(t.Context(), old, 1, 1); err != nil {
					t.Fatal(err)
				}
				if err = x.s.store.DeleteDownload(t.Context(), x.d.UserID, x.d.SessionHash, x.d.ID); err != nil {
					t.Fatal(err)
				}
				x.d, _, err = x.s.store.CreateDownload(t.Context(), x.user, x.d.SessionHash, x.d.ItemID, "large", *old)
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
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				req, _ := http.NewRequestWithContext(ctx, "GET", x.server.URL+"/api/v1/downloads/"+x.d.ID+"/assets/"+x.d.Revision, nil)
				req.Header.Set("Authorization", "Bearer "+x.token)
				res, err := x.server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				defer res.Body.Close()
				if res.StatusCode != 200 {
					t.Fatalf("transfer %d", res.StatusCode)
				}
				// Leave the body unread until the permission/source change; server writes can block.
				switch change {
				case "grant":
					no := false
					err = x.s.store.UpdateUser(t.Context(), x.user, store.UserUpdate{AllowDownloads: &no})
				case "release":
					err = x.s.store.DeleteDownload(t.Context(), x.d.UserID, x.d.SessionHash, x.d.ID)
				case "session":
					err = x.s.store.DeleteSession(t.Context(), x.d.SessionHash)
				case "source":
					err = os.Chtimes(x.d.SourcePath, time.Now(), time.Now().Add(2*time.Second))
				}
				if err != nil {
					t.Fatal(err)
				}
				started := time.Now()
				n, err := io.Copy(io.Discard, res.Body)
				if err == nil || n >= x.d.Size || time.Since(started) > 2*time.Second {
					t.Fatalf("stream not promptly aborted: bytes%d err%v took%v", n, err, time.Since(started))
				}
				if release, ok := x.s.store.ReserveCatalogMutation(); ok {
					release()
				} else {
					t.Fatal("transfer leaked catalog reservation")
				}
			})
		}
	}
}
func TestDownloadTransferAdmissionIsIndependentAndBounded(t *testing.T) {
	x := downloadTransferFixture(t, false)
	release, ok := x.m.BeginTransfer(x.d.UserID, x.d.SessionHash)
	if !ok {
		t.Fatal("admission failed")
	}
	defer release()
	res, _ := x.request(t, "GET", "/assets/"+x.d.Revision, "", "")
	if res.StatusCode != 429 {
		t.Fatalf("duplicate transfer %d", res.StatusCode)
	}
	res, _ = x.request(t, "HEAD", "/assets/"+x.d.Revision, "", "")
	if res.StatusCode != 200 {
		t.Fatal("HEAD consumed transfer slot")
	}
	var releases []func()
	for i := 0; i < 3; i++ {
		done, ok := x.m.BeginTransfer("other", strconv.Itoa(i))
		if !ok {
			t.Fatal("capacity too low")
		}
		releases = append(releases, done)
	}
	defer func() {
		for _, done := range releases {
			done()
		}
	}()
	if _, ok := x.m.BeginTransfer("overflow", "session"); ok {
		t.Fatal("server bound exceeded")
	}
}

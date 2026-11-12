package downloads

import (
	"ams/internal/artwork"
	"ams/internal/metadata"
	"ams/internal/store"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bundledFixture(t *testing.T) (*store.Store, *store.Download, *Manager, Bundle) {
	t.Helper()
	st, d, m, _ := fixture(t)
	root, uploads, artdir := t.TempDir(), t.TempDir(), t.TempDir()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploads, "poster.png"), data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	poster := metadata.UploadPrefix + "poster.png"
	backdrop := "https://fixture.invalid/already-cached.png"
	if err := st.EditItem(t.Context(), d.ItemID, store.ItemEdit{Poster: &poster, Backdrop: &backdrop}); err != nil {
		t.Fatal(err)
	}
	key := artwork.Key(artwork.Source("movie", "backdrop", backdrop))
	if err := os.MkdirAll(filepath.Join(artdir, key[:2]), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artdir, key[:2], key), data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	stem := strings.TrimSuffix(d.SourcePath, filepath.Ext(d.SourcePath))
	if err := os.WriteFile(stem+".en.srt", []byte("1\n00:00:01,000 --> 00:00:02,000\nGenerated caption\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stem+".fr.ass", []byte("[Script Info]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m.ConfigureBundles(root, uploads, artwork.New(artdir, m.log), nil)
	if err := m.Prepare(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	saved, err := st.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
	if err != nil || saved.State != "ready" {
		t.Fatalf("bundle not ready: %+v %v", saved, err)
	}
	var b Bundle
	if err = json.Unmarshal([]byte(saved.Snapshot), &b); err != nil {
		t.Fatal(err)
	}
	return st, saved, m, b
}
func TestBundleCopiesCachedArtworkTextCaptionsAndDigests(t *testing.T) {
	_, d, m, b := bundledFixture(t)
	if len(b.Assets) != 3 {
		t.Fatalf("expected poster/backdrop/caption: %+v", b.Assets)
	}
	caption := false
	unsupported := false
	for _, a := range b.Assets {
		f, err := m.OpenBundleAsset(d, a)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(f.Name())
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != a.SHA256 || int64(len(data)) != a.Size {
			t.Fatal("asset digest/size mismatch")
		}
		if a.Role == "subtitle" {
			caption = bytes.Contains(data, []byte("Generated caption"))
		}
	}
	for _, a := range b.Unavailable {
		if a.Role == "subtitle" && a.Reason == "format_or_count_unsupported" {
			unsupported = true
		}
	}
	if !caption || !unsupported {
		t.Fatal("caption/unsupported format not represented")
	}
	for _, secret := range []string{d.SourcePath, d.SessionHash, d.Key, "fixture.invalid", metadata.UploadPrefix} {
		if strings.Contains(d.Snapshot, secret) {
			t.Fatal("private snapshot value", secret)
		}
	}
}
func TestBundleCleanupRespectsReadersAndForeignKeyCascade(t *testing.T) {
	st, d, m, b := bundledFixture(t)
	release := m.LeaseBundle(d)
	if err := st.DeleteSession(t.Context(), d.SessionHash); err != nil {
		t.Fatal(err)
	}
	if err := m.CleanupBundles(t.Context()); err != nil {
		t.Fatal(err)
	}
	f, err := m.OpenBundleAsset(d, b.Assets[0])
	if err != nil {
		t.Fatal("active reader evicted", err)
	}
	f.Close()
	release()
	if err = m.CleanupBundles(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err = m.OpenBundleAsset(d, b.Assets[0]); err == nil {
		t.Fatal("orphan retained")
	}
}
func TestBundleSnapshotSurvivesReopenButDoesNotInventMissingFiles(t *testing.T) {
	st, d, m, b := bundledFixture(t)
	backup := filepath.Join(t.TempDir(), "copy.db")
	if err := st.BackupTo(t.Context(), backup); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Open(t.Context(), backup, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	saved, err := restored.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
	if err != nil || saved.Snapshot != d.Snapshot {
		t.Fatal("snapshot lost", err)
	}
	fresh := New(restored, m.log)
	fresh.ConfigureBundles(t.TempDir(), t.TempDir(), nil, nil)
	if _, err = fresh.OpenBundleAsset(saved, b.Assets[0]); err == nil {
		t.Fatal("DB restore invented artifacts")
	}
}
func TestBundlePublicationFailureDoesNotPublishReadyDigest(t *testing.T) {
	st, d, m, _ := fixture(t)
	bad := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(bad, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	m.ConfigureBundles(bad, t.TempDir(), nil, nil)
	if err := m.Prepare(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	got, err := st.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
	if err != nil || got.State != "failed" || got.SHA256 != "" || got.Snapshot != "" {
		t.Fatalf("partial bundle: %+v %v", got, err)
	}
}
func TestBundleStorageBudgetRejectsPublication(t *testing.T) {
	st, d, m, _ := fixture(t)
	root := t.TempDir()
	f, err := os.Create(filepath.Join(root, "budget-fixture"))
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate((256 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	m.ConfigureBundles(root, t.TempDir(), nil, nil)
	if err = m.Prepare(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	got, err := st.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
	if err != nil || got.State != "failed" || got.Snapshot != "" {
		t.Fatalf("over-budget publication: %+v %v", got, err)
	}
}

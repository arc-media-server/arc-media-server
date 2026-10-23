package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ams/internal/probe"
	"ams/internal/store"
)

func TestFormatBackfillIsOnceAndPreservesBasicFileData(t *testing.T) {
	ctx := context.Background()
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{"Example (2000).mkv": "isolated fixture"})
	path := filepath.Join(root, "Example (2000).mkv")
	f, err := st.GetFileByPath(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	f.Streams = []store.Stream{{Index: 0, Kind: "video", Codec: "hevc", ColorTransfer: "smpte2084"}, {Index: 1, Kind: "audio", Codec: "truehd", Channels: 8}}
	f.DurationMS = 120000
	f.Chapters = []store.Chapter{{StartMS: 0, EndMS: 1000, Title: "Keep me"}}
	if _, err = st.SaveFile(ctx, f, 1, 1); err != nil {
		t.Fatal(err)
	}
	s.prober = probe.New("nonexistent-format-ffprobe")
	if err = s.enrichPendingFormats(ctx, &store.Library{ID: lib}, 1); err != nil {
		t.Fatal(err)
	}
	pending, err := st.PendingFormats(ctx, lib, 0)
	if err != nil || len(pending) != 0 {
		t.Fatalf("failed optional probe keeps retrying: %v %v", pending, err)
	}
	if got := s.Status(lib).FilesProbed; got != 1 {
		t.Fatalf("actual read attempts=%d", got)
	}
	if err = s.enrichPendingFormats(ctx, &store.Library{ID: lib}, 1); err != nil {
		t.Fatal(err)
	}
	if got := s.Status(lib).FilesProbed; got != 1 {
		t.Fatalf("unchanged rescan reads again: %d", got)
	}
	saved, err := st.GetFileByPath(ctx, path)
	if err != nil || saved.DurationMS != 120000 || !saved.Probed || saved.ProbeError != "" {
		t.Fatalf("lost basic file: %+v %v", saved, err)
	}
	chapters, err := st.FileChapters(ctx, f.ID)
	if err != nil || len(chapters) != 1 || chapters[0].Title != "Keep me" {
		t.Fatalf("lost chapters: %v %v", chapters, err)
	}
	// A changed file is not enriched using the previous probe's revision.
	f.FormatProbeVersion = 0
	if _, err = st.SaveFile(ctx, f, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("changed size"), 0644); err != nil {
		t.Fatal(err)
	}
	if err = s.enrichPendingFormats(ctx, &store.Library{ID: lib}, 1); err != nil {
		t.Fatal(err)
	}
	pending, err = st.PendingFormats(ctx, lib, 0)
	if err != nil || len(pending) != 1 {
		t.Fatalf("changed file not left for normal probing: %v %v", pending, err)
	}
	// Restore the fixture for libraryFixture's source-preservation assertion.
	if err = os.WriteFile(path, []byte("isolated fixture"), 0644); err != nil {
		t.Fatal(err)
	}
}

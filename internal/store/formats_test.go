package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
)

func TestFormatUpgradeBackfillAndPersistence(t *testing.T) {
	ctx := context.Background()
	old := oldBackupThrough(t, 46)
	legacy, err := sql.Open("sqlite", old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`INSERT INTO media_files (id,library_id,path,size,mtime,probed_at,last_seen_at,duration_ms) VALUES (91,1,'legacy.mkv',100,2,3,4,555);
 INSERT INTO media_streams(file_id,idx,kind,codec,color_transfer) VALUES (91,0,'video','hevc','smpte2084');
 INSERT INTO media_streams(file_id,idx,kind,codec,profile,channels) VALUES (91,1,'audio','truehd','Dolby TrueHD + Dolby Atmos',8);
 INSERT INTO media_chapters(file_id,idx,start_ms,end_ms,title) VALUES (91,0,0,500,'Legacy chapter')`)
	legacy.Close()
	if err != nil {
		t.Fatal(err)
	}
	st, err := Open(ctx, old, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	oldFile, err := st.GetFileByPath(ctx, "legacy.mkv")
	if err != nil || !oldFile.Probed || oldFile.DurationMS != 555 {
		t.Fatalf("upgrade discarded probe: %+v %v", oldFile, err)
	}
	oldStreams, err := st.fileStreams(ctx, 91)
	if err != nil || len(oldStreams) != 2 || oldStreams[1].Profile != "Dolby TrueHD + Dolby Atmos" || oldStreams[0].DolbyVision {
		t.Fatalf("upgrade changed streams: %+v %v", oldStreams, err)
	}
	oldChapters, err := st.FileChapters(ctx, 91)
	if err != nil || len(oldChapters) != 1 || oldChapters[0].Title != "Legacy chapter" {
		t.Fatalf("upgrade changed chapters: %+v %v", oldChapters, err)
	}
	lib, err := st.SyncLibrary(ctx, "Formats", "movies", "tmdb_aired", []string{filepath.Join(t.TempDir(), "media")})
	if err != nil {
		t.Fatal(err)
	}
	f := MediaFile{LibraryID: lib, Path: "movie.mkv", Size: 100, ModTime: 7, DurationMS: 9000, Chapters: []Chapter{{StartMS: 0, EndMS: 1000, Title: "Chapter"}}, Streams: []Stream{{Index: 0, Kind: "video", Codec: "hevc", ColorTransfer: "smpte2084"}, {Index: 2, Kind: "audio", Codec: "truehd", Profile: "Dolby TrueHD + Dolby Atmos", Channels: 8, ChannelLayout: "7.1"}}}
	f.ID, err = st.SaveFile(ctx, &f, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := st.PendingFormatPage(ctx, lib, 0)
	if err != nil || len(pending) != 1 {
		t.Fatalf("%+v %v", pending, err)
	}
	f.Streams[0].DolbyVision = true
	f.Streams[0].DolbyVisionProfile = 8
	f.Streams[0].HDR10Plus = true
	f.Streams[0].HDRProbeStatus = "sampled"
	if err := st.SaveFormats(ctx, f); err != nil {
		t.Fatal(err)
	}
	pending, err = st.PendingFormatPage(ctx, lib, 0)
	if err != nil || len(pending) != 0 {
		t.Fatalf("repeated enrichment: %v %v", pending, err)
	}
	streams, err := st.fileStreams(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !streams[0].DolbyVision || !streams[0].HDR10Plus || streams[0].DolbyVisionProfile != 8 || streams[1].ChannelLayout != "7.1" || streams[1].Profile != f.Streams[1].Profile {
		t.Fatalf("lost evidence: %+v", streams)
	}
	chapters, err := st.FileChapters(ctx, f.ID)
	if err != nil || len(chapters) != 1 || chapters[0].Title != "Chapter" {
		t.Fatalf("changed chapters: %v %v", chapters, err)
	}
	// A normal new-format probe also persists positive evidence (not just backfill).
	f.FormatProbeVersion = 1
	if _, err := st.SaveFile(ctx, &f, 2, 3); err != nil {
		t.Fatal(err)
	}
	streams, err = st.fileStreams(ctx, f.ID)
	if err != nil || !streams[0].DolbyVision || !streams[0].HDR10Plus || streams[0].HDRProbeStatus != "sampled" {
		t.Fatalf("normal probe lost evidence: %+v %v", streams, err)
	}
	// A regular new probe wins over an old in-flight enrichment.
	f.FormatProbeVersion = 1
	f.Streams[0].DolbyVision = false
	f.Streams[0].HDR10Plus = false
	if _, err := st.SaveFile(ctx, &f, 3, 4); err != nil {
		t.Fatal(err)
	}
	stale := f
	stale.Streams = []Stream{{Index: 0, Kind: "video", DolbyVision: true}}
	if err := st.SaveFormats(ctx, stale); err != nil {
		t.Fatal(err)
	}
	streams, _ = st.fileStreams(ctx, f.ID)
	if streams[0].DolbyVision {
		t.Fatal("stale enrichment overwrote regular probe")
	}
}

func TestPendingFormatPagesIncludePreviouslySeenFiles(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "formats.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(ctx, "Formats", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 35; i++ {
		seen := int64(2)
		if i == 0 {
			seen = 1
		}
		f := MediaFile{LibraryID: lib, Path: fmt.Sprintf("%02d.mkv", i), Streams: []Stream{{Index: 0, Kind: "video", Codec: "hevc"}}}
		if _, err := st.SaveFile(ctx, &f, 1, seen); err != nil {
			t.Fatal(err)
		}
	}
	files, err := st.PendingFormatPage(ctx, lib, 0)
	if err != nil || len(files) != 32 {
		t.Fatalf("batch=%d err=%v", len(files), err)
	}
	for _, f := range files {

		if err := st.SaveFormats(ctx, f); err != nil {
			t.Fatal(err)
		}
	}
	files, err = st.PendingFormatPage(ctx, lib, 0)
	if err != nil || len(files) != 3 {
		t.Fatalf("remaining batch=%d err=%v", len(files), err)
	}
}

// Version 2 added aspect ratios: every video read before it, HDR or not, is brought up to date
// once; current reads and files without video are left alone.
func TestPendingFormatPagesBackfillAspectRatios(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "formats.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(ctx, "Formats", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	save := func(path string, version int, streams ...Stream) MediaFile {
		f := MediaFile{LibraryID: lib, Path: path, Size: 1, ModTime: 1, FormatProbeVersion: version, Streams: streams}
		if f.ID, err = st.SaveFile(ctx, &f, 1, 1); err != nil {
			t.Fatal(err)
		}
		return f
	}
	old := save("sdr-v1.mkv", 1, Stream{Index: 0, Kind: "video", Codec: "h264", Width: 720, Height: 480},
		Stream{Index: 1, Kind: "audio", Codec: "aac"})
	hdr := save("hdr-v1.mkv", 1, Stream{Index: 0, Kind: "video", Codec: "hevc", DolbyVision: true, DolbyVisionProfile: 8, HDRProbeStatus: "sampled"})
	save("current.mkv", FormatVersion, Stream{Index: 0, Kind: "video", Codec: "h264", AspectRatio: 1.78})
	save("audio-only.mka", 0, Stream{Index: 0, Kind: "audio", Codec: "flac"})

	files, err := st.PendingFormatPage(ctx, lib, 0)
	if err != nil || len(files) != 2 || files[0].ID != old.ID || files[1].ID != hdr.ID {
		t.Fatalf("pending: %+v %v", files, err)
	}
	if !files[1].Streams[0].DolbyVision || files[1].Streams[0].DolbyVisionProfile != 8 {
		t.Fatalf("pending page lost saved HDR details: %+v", files[1].Streams)
	}
	files[0].Streams[0].AspectRatio = 1.78
	if err := st.SaveFormats(ctx, files[0]); err != nil {
		t.Fatal(err)
	}
	streams, err := st.fileStreams(ctx, old.ID)
	if err != nil || streams[0].AspectRatio != 1.78 {
		t.Fatalf("aspect ratio not saved: %+v %v", streams, err)
	}
	if files, err = st.PendingFormatPage(ctx, lib, 0); err != nil || len(files) != 1 || files[0].ID != hdr.ID {
		t.Fatalf("after backfill: %+v %v", files, err)
	}
}

func TestFormatEnrichmentRejectsDifferentRestoredPath(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "formats.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(ctx, "Formats", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	f := MediaFile{LibraryID: lib, Path: "restored.mkv", Size: 100, ModTime: 1, Streams: []Stream{{Index: 0, Kind: "video", Codec: "hevc"}}}
	f.ID, err = st.SaveFile(ctx, &f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	stale := f
	stale.Path = "different-before-restore.mkv"
	stale.Streams = []Stream{{Index: 0, Kind: "video", Codec: "hevc", DolbyVision: true}}
	if err := st.SaveFormats(ctx, stale); err != nil {
		t.Fatal(err)
	}
	pending, err := st.PendingFormatPage(ctx, lib, 0)
	if err != nil || len(pending) != 1 || pending[0].Streams[0].DolbyVision {
		t.Fatalf("stale restored path changed source evidence: %+v %v", pending, err)
	}
}

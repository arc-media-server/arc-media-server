package store

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDVCompatibilitySurvivesStorageAndExplicitZero(t *testing.T) {
	st, err := Open(t.Context(), filepath.Join(t.TempDir(), "db.sqlite"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []int{0, 1, 2, 4, 6} {
		f := MediaFile{LibraryID: lib, Path: t.TempDir() + "/sample.mkv", Streams: []Stream{{Index: 0, Kind: "video", Codec: "hevc", DolbyVision: true, DolbyVisionProfile: 8, DVBLCompatibilityID: &value}}}
		id, err := st.SaveFile(t.Context(), &f, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		streams, err := st.fileStreams(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		if len(streams) != 1 || streams[0].DVBLCompatibilityID == nil || *streams[0].DVBLCompatibilityID != value {
			t.Fatalf("lost compatibility: %+v %v", streams, err)
		}
		data, _ := json.Marshal(streams[0])
		if value == 0 && !strings.Contains(string(data), `"dv_bl_signal_compatibility_id":0`) {
			t.Fatal("invalid response")
		}
	}
}

package scanner

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestFailedBackgroundScanLogsLibraryAndStage(t *testing.T) {
	_, s, lib, _ := libraryFixture(t, "movies", map[string]string{})
	var output bytes.Buffer
	s.log = slog.New(slog.NewJSONHandler(&output, nil))
	// Reject an out-of-library request through the real background scan path.
	s.RequestPaths(lib, []string{t.TempDir()})
	s.Wait()
	if s.Status(lib).Error == "" {
		t.Fatal("fixture did not produce a scan failure")
	}
	decoder := json.NewDecoder(&output)
	for decoder.More() {
		var entry struct {
			Message string `json:"msg"`
			Library string `json:"library"`
			ID      int64  `json:"library_id"`
			Phase   string `json:"phase"`
			Error   string `json:"err"`
		}
		if err := decoder.Decode(&entry); err != nil {
			t.Fatal(err)
		}
		if entry.Message == "scan failed" {
			if entry.Library != "Fixture" || entry.ID != lib || entry.Phase != "Checking library folders" || entry.Error != s.Status(lib).Error {
				t.Fatalf("failure lost its context: %+v", entry)
			}
			return
		}
	}
	t.Fatal("no scan failure logged")
}

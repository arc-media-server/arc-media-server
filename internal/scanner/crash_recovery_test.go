package scanner

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/store"
)

func TestKilledScannerProcessRecovers(t *testing.T) {
	ctx := context.Background()
	if path := os.Getenv("AMS_CRASH_FIXTURE"); path != "" {
		st, err := store.Open(ctx, path, store.Options{})
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		lib, err := strconv.ParseInt(os.Getenv("AMS_CRASH_LIBRARY"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		s := New(ctx, st, probe.New("missing-test-ffprobe"), metadata.NewTMDB(""), metadata.NewTVDB(""), slog.New(slog.DiscardHandler))
		s.Start(lib)
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if s.Status(lib).Phase == "Reading file details" {
				fmt.Println("SCAN_AT_PROBE")
				s.Wait() // Parent kills this process while the delayed probe is pending.
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("scan did not reach probe phase")
	}
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{"Alien (1979).mkv": "synthetic file"})
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	items, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "movie"})
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	id := items[0].ID
	title := "Keep this title"
	if err := st.EditItem(ctx, id, store.ItemEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReportProgress(ctx, u.ID, id, 120000, 7200000); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Alien (1979).mkv")
	f, err := st.GetFileByPath(ctx, path)
	if err != nil || f == nil {
		t.Fatalf("file=%v err=%v", f, err)
	}
	if _, err := st.SaveFile(ctx, f, 0, 1); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(filepath.Dir(root), "ams.db")
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	processCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(processCtx, bin, "-test.run=^TestKilledScannerProcessRecovers$")
	for _, e := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(e), "AMS_") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	cmd.Env = append(cmd.Env, "AMS_CRASH_FIXTURE="+dbPath, fmt.Sprintf("AMS_CRASH_LIBRARY=%d", lib), "AMS_PROBE_DELAY_MS=60000")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
	ready := make(chan bool, 1)
	go func() {
		lines := bufio.NewScanner(stdout)
		for lines.Scan() {
			if lines.Text() == "SCAN_AT_PROBE" {
				ready <- true
				return
			}
		}
		ready <- false
	}()
	select {
	case ok := <-ready:
		if !ok {
			cmd.Wait()
			t.Fatal("child exited before scan interruption point")
		}
	case <-time.After(15 * time.Second):
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatal("child did not reach interruption point")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("child unexpectedly exited cleanly")
	}
	reopened, err := store.Open(ctx, dbPath, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted := New(ctx, reopened, probe.New("missing-test-ffprobe"), metadata.NewTMDB(""), metadata.NewTVDB(""), slog.New(slog.DiscardHandler))
	restarted.status[lib] = &Status{}
	if err := restarted.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	file, err := reopened.GetFileByPath(ctx, path)
	if err != nil || file == nil || file.ID != f.ID || file.Missing {
		t.Fatalf("file lost: %+v %v", file, err)
	}
	it, err := reopened.GetItem(ctx, id)
	if err != nil || it == nil || it.Title != title {
		t.Fatalf("edit lost: %+v %v", it, err)
	}
	data, err := reopened.UserDataFor(ctx, u.ID, []string{id})
	if err != nil || data[id].PositionMS != 120000 {
		t.Fatalf("resume lost: %+v %v", data, err)
	}
	if err := store.CheckBackup(ctx, dbPath); err != nil {
		t.Fatal(err)
	}
}

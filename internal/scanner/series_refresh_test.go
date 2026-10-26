package scanner

import (
	"context"
	"errors"
	"testing"
)

func TestScopedRefreshLeaseStopAndPendingDiscovery(t *testing.T) {
	st, s, lib, _ := libraryFixture(t, "shows", map[string]string{"Example/Example S01E01.mkv": "episode"})
	library, err := st.GetLibrary(t.Context(), lib)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- s.RefreshSeries(context.Background(), library, "fixture", false, func(ctx context.Context) error { close(entered); <-ctx.Done(); return ctx.Err() })
	}()
	<-entered
	old := s.Status(lib).JobID
	err = s.RefreshSeries(t.Context(), library, "fixture", false, func(context.Context) error { t.Error("busy refresh mutated settings"); return nil })
	if !errors.Is(err, ErrLibraryBusy) {
		t.Fatalf("busy=%v", err)
	}
	s.Request(lib)
	if !s.Stop(lib, old) {
		t.Fatal("could not stop scoped refresh")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
	s.Wait()
	if got := s.Status(lib); got.Running || !got.Stopped || got.FilesSeen != 0 {
		t.Fatalf("scoped refresh/queued discovery escaped Stop: %+v", got)
	}
	closed, cancel := context.WithCancel(context.Background())
	cancel()
	s.ctx = closed
	if err := s.RefreshSeries(t.Context(), library, "fixture", false, func(context.Context) error {
		t.Error("refresh mutated settings after scanner shutdown")
		return nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown=%v", err)
	}
}

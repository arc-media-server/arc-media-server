package downloads

import (
	"context"
	"testing"
	"time"
)

func TestPreparationReservesCatalogThroughPublication(t *testing.T) {
	st, _, m, _ := fixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	entered := make(chan struct{}, 1)
	m.Playback = func() bool {
		select {
		case entered <- struct{}{}:
		default:
		}
		return true
	}
	done := make(chan struct{})
	go func() { defer close(done); m.Run(ctx) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("preparation did not enter")
	}
	release, ok := st.ReserveCatalogMutation()
	if ok {
		release()
		cancel()
		<-done
		t.Fatal("restore can cross active preparation")
	}
	cancel()
	<-done
	release, ok = st.ReserveCatalogMutation()
	if !ok {
		t.Fatal("preparation leaked catalog lease")
	}
	release()
}
func TestPreparationWaitsForCatalogMaintenanceWithoutFailingRecord(t *testing.T) {
	st, d, m, _ := fixture(t)
	release, ok := st.ReserveCatalogMutation()
	if !ok {
		t.Fatal("could not reserve maintenance")
	}
	if err := m.Prepare(t.Context(), d); err != ErrCatalogBusy {
		release()
		t.Fatalf("busy prepare: %v", err)
	}
	got, err := st.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
	if err != nil || got.State != "preparing" {
		release()
		t.Fatal("maintenance failed record", err)
	}
	release()
	if err = m.Prepare(t.Context(), d); err != nil {
		t.Fatal(err)
	}
}

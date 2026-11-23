package store

import (
	"fmt"
	"testing"
	"time"
)

func TestClientLogRetention(t *testing.T) {
	s := openTest(t)
	ctx := t.Context()
	now := time.Now()
	newDevice := func(id string) {
		t.Helper()
		if err := s.CreateSession(ctx, id, "", "password", ""); err != nil {
			t.Fatal(err)
		}
		if err := s.SetSessionDevice(ctx, id, id); err != nil {
			t.Fatal(err)
		}
	}
	newDevice("first")
	newDevice("second")
	send := func(device string, at time.Time) {
		t.Helper()
		ok, err := s.SaveClientLogs(ctx, device, "kodi", []ClientLogEntry{{TimeMS: at.UnixMilli(), Level: "info", Event: "app_started"}}, at)
		if !ok || err != nil {
			t.Fatalf("upload rejected: %v", err)
		}
	}
	send("second", now.Add(-clientLogRetention))
	for range clientLogsPerDevice + 1 {
		send("first", now)
	}
	devices, err := s.ListClientLogDevices(ctx, now)
	if err != nil || len(devices) != 1 || devices[0].Device != "first" {
		t.Fatalf("unexpected devices: %+v %v", devices, err)
	}
	batches, err := s.ClientLogs(ctx, devices[0].ID, now)
	if err != nil || len(batches) != clientLogsPerDevice {
		t.Fatalf("unexpected batch count %d: %v", len(batches), err)
	}
	for i := range clientLogsTotal + 1 {
		id := fmt.Sprint(i)
		newDevice(id)
		send(id, now)
	}
	devices, err = s.ListClientLogDevices(ctx, now)
	if err != nil || len(devices) != clientLogsTotal {
		t.Fatalf("unexpected global count %d: %v", len(devices), err)
	}
	future := now.Add(clientLogRetention)
	batches, err = s.ClientLogs(ctx, devices[0].ID, future)
	if err != nil || len(batches) > 0 {
		t.Fatalf("expired entries visible: %v", err)
	}
	if count, err := s.PruneClientLogs(ctx, future); err != nil || count != clientLogsTotal {
		t.Fatalf("unexpected prune count %d: %v", count, err)
	}
}

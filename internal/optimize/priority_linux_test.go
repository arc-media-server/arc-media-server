package optimize

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestOptimizeLowersAllChildThreadsWithoutChangingServerPriority(t *testing.T) {
	x := conversionTest(t)
	helperConverter(t, x)
	parent, err := unix.Getpriority(unix.PRIO_PROCESS, os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	o := x.queue(t, 720)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- x.m.convert(ctx, o, nil) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("worker did not stop")
		}
	}()
	waitReady(t, ctx, x.root)
	raw, err := os.ReadFile(filepath.Join(x.root, "ready"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	var wrong bool
	for {
		tasks, err := os.ReadDir("/proc/" + strconv.Itoa(pid) + "/task")
		if err != nil {
			t.Fatal(err)
		}
		wrong = false
		for _, task := range tasks {
			tid, _ := strconv.Atoi(task.Name())
			raw, err := unix.Getpriority(unix.PRIO_PROCESS, tid)
			if err == unix.ESRCH {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			// Linux returns20-nice, unlike the public nice range. Every worker must be at least+10.
			if 20-raw < 10 {
				wrong = true
			}
		}
		if !wrong || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if wrong {
		t.Error("optimized worker retained normal-priority threads")
	}

	// Never undo an even lower priority inherited from the deployment environment.
	tasks, err := os.ReadDir("/proc/" + strconv.Itoa(pid) + "/task")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		tid, _ := strconv.Atoi(task.Name())
		if err := unix.Setpriority(unix.PRIO_PROCESS, tid, 15); err != nil && err != unix.ESRCH {
			t.Fatal(err)
		}
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Release()
	if err := lowerOptimizePriority(process); err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		tid, _ := strconv.Atoi(task.Name())
		raw, err := unix.Getpriority(unix.PRIO_PROCESS, tid)
		if err == unix.ESRCH {
			continue
		}
		if err != nil || 20-raw < 15 {
			t.Fatalf("inherited lower priority raised: raw=%d err=%v", raw, err)
		}
	}
	after, err := unix.Getpriority(unix.PRIO_PROCESS, os.Getpid())
	if err != nil || after != parent {
		t.Fatalf("server priority changed: before=%d after=%d err=%v", parent, after, err)
	}
}

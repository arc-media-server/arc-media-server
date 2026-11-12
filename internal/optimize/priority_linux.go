package optimize

import (
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// Linux nice is per-thread. Lower the leader first so new FFmpeg workers inherit it,
// then cover threads that already existed when Start returned. Never raise a priority
// inherited from a host that already runs Arc below our normal background target.
func lowerOptimizePriority(p *os.Process) error {
	lower := func(tid int) error {
		raw, err := unix.Getpriority(unix.PRIO_PROCESS, tid)
		if err == unix.ESRCH {
			return nil // exited before the hint
		}
		if err != nil {
			return err
		}
		// Linux's raw syscall returns 20-nice, unlike the public nice range.
		if 20-raw >= 10 {
			return nil
		}
		if err := unix.Setpriority(unix.PRIO_PROCESS, tid, 10); err != unix.ESRCH {
			return err
		}
		return nil
	}
	if err := lower(p.Pid); err != nil {
		return err
	}
	// A second snapshot covers initialization threads created during the first pass.
	for range 2 {
		tasks, err := os.ReadDir("/proc/" + strconv.Itoa(p.Pid) + "/task")
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, task := range tasks {
			tid, err := strconv.Atoi(task.Name())
			if err != nil {
				continue
			}
			if err := lower(tid); err != nil {
				return err
			}
		}
	}
	return nil
}

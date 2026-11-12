package optimize

import (
	"os"

	"golang.org/x/sys/unix"
)

func lowerOptimizePriority(p *os.Process) error {
	priority, err := unix.Getpriority(unix.PRIO_PROCESS, p.Pid)
	if err == unix.ESRCH {
		return nil
	}
	if err != nil {
		return err
	}
	if priority >= 10 {
		return nil
	}
	if err := unix.Setpriority(unix.PRIO_PROCESS, p.Pid, 10); err != unix.ESRCH {
		return err
	}
	return nil
}

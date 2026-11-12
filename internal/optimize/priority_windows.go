package optimize

import (
	"os"

	"golang.org/x/sys/windows"
)

func lowerOptimizePriority(p *os.Process) error {
	h, err := windows.OpenProcess(windows.PROCESS_SET_INFORMATION|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(p.Pid))
	if err == windows.ERROR_INVALID_PARAMETER {
		return nil
	} // already exited
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	priority, err := windows.GetPriorityClass(h)
	if err != nil {
		return err
	}
	if priority == windows.IDLE_PRIORITY_CLASS || priority == windows.BELOW_NORMAL_PRIORITY_CLASS {
		return nil
	}
	return windows.SetPriorityClass(h, windows.BELOW_NORMAL_PRIORITY_CLASS)
}

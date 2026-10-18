//go:build !windows && !linux

package sysstats

import (
	"runtime"
	"syscall"
	"time"
)

// Elsewhere (macOS, BSD) only Arc's own CPU time and memory are read; the whole computer's show as
// unknown.
func readCPU() (cpuTimes, error) {
	var c cpuTimes
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return c, err
	}
	c.arc = time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
	return c, nil
}

func readMemory() (memory, error) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return memory{arc: int64(ms.Sys), used: -1, total: -1}, nil
}

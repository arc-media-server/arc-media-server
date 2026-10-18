package sysstats

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemTimes       = kernel32.NewProc("GetSystemTimes")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetProcessMemoryInfo = kernel32.NewProc("K32GetProcessMemoryInfo")
)

func filetime(ft syscall.Filetime) time.Duration {
	return time.Duration(int64(ft.HighDateTime)<<32|int64(ft.LowDateTime)) * 100
}

func readCPU() (cpuTimes, error) {
	var c cpuTimes
	var created, exited, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(syscall.Handle(^uintptr(0)), &created, &exited, &kernel, &user); err != nil {
		return c, err
	}
	c.arc = filetime(kernel) + filetime(user)
	var idle, sysKernel, sysUser syscall.Filetime
	if r, _, _ := procGetSystemTimes.Call(uintptr(unsafe.Pointer(&idle)), uintptr(unsafe.Pointer(&sysKernel)), uintptr(unsafe.Pointer(&sysUser))); r != 0 {
		// Kernel time includes idle time.
		c.total = filetime(sysKernel) + filetime(sysUser)
		c.busy = c.total - filetime(idle)
		c.haveSystem = true
	}
	return c, nil
}

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

type processMemoryCounters struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

func readMemory() (memory, error) {
	var m memory
	st := memoryStatusEx{Length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if r, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&st))); r == 0 {
		return m, err
	}
	m.total, m.used = int64(st.TotalPhys), int64(st.TotalPhys-st.AvailPhys)
	pc := processMemoryCounters{Cb: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	if r, _, _ := procGetProcessMemoryInfo.Call(uintptr(^uintptr(0)), uintptr(unsafe.Pointer(&pc)), uintptr(pc.Cb)); r != 0 {
		m.arc = int64(pc.WorkingSetSize)
	} else {
		m.arc = -1
	}
	return m, nil
}

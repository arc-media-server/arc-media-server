package transcode

import (
	"os"
	"syscall"
)

// Windows has no stop signal; ntdll's NtSuspendProcess/NtResumeProcess pause a whole process.
var (
	ntdll            = syscall.NewLazyDLL("ntdll.dll")
	ntSuspendProcess = ntdll.NewProc("NtSuspendProcess")
	ntResumeProcess  = ntdll.NewProc("NtResumeProcess")
)

const processSuspendResume = 0x0800

func suspend(p *os.Process) error { return callOnProcess(ntSuspendProcess, p) }
func resume(p *os.Process) error  { return callOnProcess(ntResumeProcess, p) }

func callOnProcess(proc *syscall.LazyProc, p *os.Process) error {
	if err := proc.Find(); err != nil {
		return err
	}
	h, err := syscall.OpenProcess(processSuspendResume, false, uint32(p.Pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)
	if status, _, _ := proc.Call(uintptr(h)); status != 0 {
		return syscall.Errno(status)
	}
	return nil
}

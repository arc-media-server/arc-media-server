//go:build !windows

package transcode

import (
	"os"
	"syscall"
)

func suspend(p *os.Process) error { return p.Signal(syscall.SIGSTOP) }
func resume(p *os.Process) error  { return p.Signal(syscall.SIGCONT) }

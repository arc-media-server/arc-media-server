package datalock

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
)

func Acquire(dir string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(dir, ".arc-database.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	var o windows.Overlapped
	if err = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &o); err != nil {
		f.Close()
		return nil, errors.New("Arc data directory is already in use; stop the other Arc process first")
	}
	return func() { windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &o); f.Close() }, nil
}

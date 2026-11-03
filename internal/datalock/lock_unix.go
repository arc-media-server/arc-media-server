//go:build !windows

// Package datalock excludes concurrent Arc processes using the same local data directory.
package datalock

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
)

func Acquire(dir string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(dir, ".arc-database.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("Arc data directory is already in use; stop the other Arc process first")
	}
	return func() { unix.Flock(int(f.Fd()), unix.LOCK_UN); f.Close() }, nil
}

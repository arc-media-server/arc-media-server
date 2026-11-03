package datalock

import "testing"

func TestDataDirectoryExcludesSecondProcessAndReleases(t *testing.T) {
	dir := t.TempDir()
	release, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	if another, err := Acquire(dir); err == nil {
		another()
		release()
		t.Fatal("data directory admitted two owners")
	}
	release()
	next, err := Acquire(dir)
	if err != nil {
		t.Fatal("released lock not reusable", err)
	}
	next()
}

//go:build !linux && !windows

package diskspace

import "errors"

func Available(string) (uint64, error) {
	return 0, errors.New("free space check is unavailable on this platform")
}

//go:build !linux && !windows && !darwin

package optimize

import (
	"errors"
	"os"
)

func lowerOptimizePriority(*os.Process) error {
	return errors.New("background CPU priority is unsupported on this platform")
}

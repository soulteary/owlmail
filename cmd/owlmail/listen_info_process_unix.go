//go:build !windows

package main

import (
	"errors"
	"fmt"
	"syscall"
)

func validateListenInfoProcess(pid int) error {
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return fmt.Errorf("listener information owner is no longer running: %w", err)
	}
	return nil
}

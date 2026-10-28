package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func validateListenInfoProcess(pid int) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("inspect listener information owner: %w", err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil || status != uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("listener information owner is no longer running")
	}
	return nil
}

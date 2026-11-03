//go:build windows

package mailbox

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// The persistent sidecar is dedicated to this writer protocol.
func lockControlWriter(file *os.File) error {
	overlapped := windows.Overlapped{}
	err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return ErrWriterActive
	}
	if err != nil {
		return fmt.Errorf("lock control writer: %w", err)
	}
	return nil
}

func singleLinkControlFile(file *os.File) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return err
	}
	if info.NumberOfLinks != 1 {
		return ErrLinkedAuthority
	}
	return nil
}

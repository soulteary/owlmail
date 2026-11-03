//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly && !windows

package mailbox

import (
	"errors"
	"os"
)

func lockControlWriter(_ *os.File) error {
	return errors.New("exclusive control writer locking is unsupported on this platform")
}

func singleLinkControlFile(_ *os.File) error {
	return errors.New("control writer file validation is unsupported on this platform")
}
